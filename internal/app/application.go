package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/soheilprs/chainwatch/internal/api"
	"github.com/soheilprs/chainwatch/internal/config"
	"github.com/soheilprs/chainwatch/internal/domain"
	"github.com/soheilprs/chainwatch/internal/ethereum"
	"github.com/soheilprs/chainwatch/internal/indexer"
	"github.com/soheilprs/chainwatch/internal/lifecycle"
	"github.com/soheilprs/chainwatch/internal/metadata"
	"github.com/soheilprs/chainwatch/internal/observability"
	"github.com/soheilprs/chainwatch/internal/store"
)

// Run composes ChainWatch's runtime dependencies and blocks until
// the service stops or the context is canceled.
func Run(ctx context.Context, runtimeConfig config.Config, logger *slog.Logger) error {
	if err := runtimeConfig.Validate(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	metrics := observability.NewMetrics()
	client, err := ethereum.NewEthereumClient(ctx, runtimeConfig.EthereumRPCURL)
	if err != nil {
		return fmt.Errorf("create Ethereum client: %w", err)
	}
	defer client.Close()

	resilientClient, err := ethereum.NewResilientEthereumClient(
		client,
		ethereumRPCConfig(runtimeConfig.RPCConfig()),
		metrics,
	)
	if err != nil {
		return fmt.Errorf("create resilient Ethereum client: %w", err)
	}

	pool, err := pgxpool.New(ctx, runtimeConfig.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	logger.InfoContext(ctx, "connected to PostgreSQL")

	persistence := store.NewPostgresBlockPersistence(pool, "ethereum_erc20")
	transferReader := store.NewPostgresTransferReader(pool)
	tokenMetadataStore := store.NewPostgresTokenMetadataStore(pool)
	tokenMetadataService := metadata.NewTokenMetadataService(resilientClient, tokenMetadataStore)

	startBlock, err := determineStartBlock(
		ctx,
		resilientClient,
		persistence,
		logger,
		runtimeConfig.ConfirmationDepth,
		runtimeConfig.MaxReorgDepth,
	)
	if err != nil {
		return err
	}

	rangeIndexer := indexer.NewConcurrentRangeIndexer(
		resilientClient,
		persistence,
		runtimeConfig.WorkerCount,
	)
	continuousIndexer := indexer.NewContinuousIndexer(
		resilientClient,
		rangeIndexer,
		startBlock,
		runtimeConfig.PollInterval,
		runtimeConfig.ConfirmationDepth,
	)
	httpAPI := api.NewHTTPServerWithObservability(
		transferReader,
		tokenMetadataService,
		logger,
		metrics,
	)
	server := &http.Server{
		Addr:              runtimeConfig.HTTPAddress,
		Handler:           httpAPI.Handler(),
		ReadHeaderTimeout: runtimeConfig.ReadHeaderTimeout,
		ReadTimeout:       runtimeConfig.ReadTimeout,
		WriteTimeout:      runtimeConfig.WriteTimeout,
		IdleTimeout:       runtimeConfig.IdleTimeout,
	}
	var profilingServer *http.Server
	if runtimeConfig.ProfilingAddress != "" {
		profilingServer = &http.Server{
			Addr:              runtimeConfig.ProfilingAddress,
			Handler:           observability.NewProfilingHandler(),
			ReadHeaderTimeout: runtimeConfig.ReadHeaderTimeout,
			ReadTimeout:       runtimeConfig.ReadTimeout,
			IdleTimeout:       runtimeConfig.IdleTimeout,
		}
		logger.InfoContext(ctx, "pprof server enabled", "address", profilingServer.Addr)
	}

	logger.InfoContext(
		ctx,
		"ChainWatch started",
		"workers", runtimeConfig.WorkerCount,
		"httpAddress", server.Addr,
	)

	services := []lifecycle.Service{
		{
			Name: "indexer",
			Run: func(serviceCtx context.Context) error {
				err := continuousIndexer.Run(serviceCtx, func(index domain.BlockTransferIndex) {
					metrics.RecordIndexedBlock(index.TransferCount())
					logger.InfoContext(
						serviceCtx,
						"indexed block",
						"blockNumber", index.BlockNumber,
						"blockHash", string(index.BlockHash),
						"transfers", index.TransferCount(),
					)
				})
				if err != nil {
					metrics.RecordIndexerError()
				}
				return err
			},
		},
		{
			Name: "HTTP server",
			Run: func(context.Context) error {
				err := server.ListenAndServe()
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			},
			Shutdown: server.Shutdown,
		},
	}
	if profilingServer != nil {
		services = append(services, lifecycle.Service{
			Name: "pprof server",
			Run: func(context.Context) error {
				err := profilingServer.ListenAndServe()
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			},
			Shutdown: profilingServer.Shutdown,
		})
	}

	runErr := lifecycle.Run(ctx, runtimeConfig.ShutdownTimeout, services...)
	if ctx.Err() != nil {
		logger.Info("shutdown signal received")
	}

	if runErr == nil {
		logger.Info("ChainWatch stopped cleanly")
	}
	return runErr
}

func ethereumRPCConfig(settings config.RPCConfig) ethereum.RPCResilienceConfig {
	return ethereum.RPCResilienceConfig{
		MaxAttempts:       settings.MaxAttempts,
		InitialBackoff:    settings.InitialBackoff,
		MaxBackoff:        settings.MaxBackoff,
		JitterFraction:    settings.JitterFraction,
		RequestsPerSecond: settings.RequestsPerSecond,
		Burst:             settings.Burst,
	}
}

func determineStartBlock(
	ctx context.Context,
	client interface {
		indexer.LatestObservedBlockClient
		indexer.CanonicalBlockClient
	},
	persistence indexer.ReorgPersistence,
	logger *slog.Logger,
	confirmationDepth uint64,
	maxReorgDepth uint64,
) (uint64, error) {
	reorgResult, err := indexer.NewReorgManager(
		client,
		persistence,
		maxReorgDepth,
	).Reconcile(ctx)
	if err != nil {
		return 0, fmt.Errorf("reconcile Ethereum chain: %w", err)
	}

	if reorgResult.ReorgDetected {
		logger.WarnContext(
			ctx,
			"Ethereum reorg rolled back",
			"previousCheckpoint", reorgResult.PreviousCheckpoint.Number,
			"commonAncestor", reorgResult.Checkpoint.Number,
			"commonAncestorHash", string(reorgResult.Checkpoint.Hash),
		)
	}

	if reorgResult.Exists {
		startBlock := reorgResult.Checkpoint.Number + 1
		logger.InfoContext(
			ctx,
			"resuming from checkpoint",
			"checkpoint", reorgResult.Checkpoint.Number,
			"startBlock", startBlock,
		)
		return startBlock, nil
	}

	latestBlock, err := client.GetLatestObservedBlock(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch latest Ethereum block: %w", err)
	}
	startBlock := uint64(0)
	if latestBlock.Number >= confirmationDepth {
		startBlock = latestBlock.Number - confirmationDepth
	}
	logger.InfoContext(
		ctx,
		"starting from latest finalized block",
		"startBlock", startBlock,
		"confirmationDepth", confirmationDepth,
	)

	return startBlock, nil
}
