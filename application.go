package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunApplication composes ChainWatch's runtime dependencies and blocks until
// the service stops or the context is canceled.
func RunApplication(ctx context.Context, config Config, logger *slog.Logger) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	metrics := NewMetrics()
	client, err := NewEthereumClient(ctx, config.EthereumRPCURL)
	if err != nil {
		return fmt.Errorf("create Ethereum client: %w", err)
	}
	defer client.Close()

	resilientClient, err := NewResilientEthereumClient(
		client,
		config.RPCResilienceConfig(),
		metrics,
	)
	if err != nil {
		return fmt.Errorf("create resilient Ethereum client: %w", err)
	}

	pool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	logger.InfoContext(ctx, "connected to PostgreSQL")

	persistence := NewPostgresBlockPersistence(pool, "ethereum_erc20")
	transferReader := NewPostgresTransferReader(pool)
	tokenMetadataStore := NewPostgresTokenMetadataStore(pool)
	tokenMetadataService := NewTokenMetadataService(resilientClient, tokenMetadataStore)

	startBlock, err := determineStartBlock(
		ctx,
		resilientClient,
		persistence,
		logger,
		config.ConfirmationDepth,
		config.MaxReorgDepth,
	)
	if err != nil {
		return err
	}

	rangeIndexer := NewConcurrentRangeIndexer(
		resilientClient,
		persistence,
		config.WorkerCount,
	)
	indexer := NewContinuousIndexer(
		resilientClient,
		rangeIndexer,
		startBlock,
		config.PollInterval,
		config.ConfirmationDepth,
	)
	api := NewHTTPServerWithObservability(
		transferReader,
		tokenMetadataService,
		logger,
		metrics,
	)
	server := &http.Server{
		Addr:              config.HTTPAddress,
		Handler:           api.Handler(),
		ReadHeaderTimeout: config.ReadHeaderTimeout,
	}

	logger.InfoContext(
		ctx,
		"ChainWatch started",
		"workers", config.WorkerCount,
		"httpAddress", server.Addr,
	)

	errCh := make(chan error, 2)
	go func() {
		errCh <- indexer.Run(ctx, func(index BlockTransferIndex) {
			metrics.RecordIndexedBlock(index.TransferCount())
			logger.InfoContext(
				ctx,
				"indexed block",
				"blockNumber", index.BlockNumber,
				"blockHash", string(index.BlockHash),
				"transfers", index.TransferCount(),
			)
		})
	}()
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errCh:
		if runErr != nil {
			metrics.RecordIndexerError()
			logger.Error("service stopped with error", "error", runErr)
		}
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		config.ShutdownTimeout,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return errors.Join(runErr, fmt.Errorf("shut down HTTP server: %w", err))
	}

	logger.Info("ChainWatch stopped cleanly")
	return runErr
}

func determineStartBlock(
	ctx context.Context,
	client interface {
		LatestObservedBlockClient
		CanonicalBlockClient
	},
	persistence ReorgPersistence,
	logger *slog.Logger,
	confirmationDepth uint64,
	maxReorgDepth uint64,
) (uint64, error) {
	reorgResult, err := NewReorgManager(
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
