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
	tokenMetadataService := NewTokenMetadataService(client, tokenMetadataStore)

	startBlock, err := determineStartBlock(ctx, client, persistence, logger)
	if err != nil {
		return err
	}

	rangeIndexer := NewConcurrentRangeIndexer(
		client,
		persistence,
		config.WorkerCount,
	)
	indexer := NewContinuousIndexer(
		client,
		rangeIndexer,
		startBlock,
		config.PollInterval,
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
	client LatestObservedBlockClient,
	persistence BlockPersistence,
	logger *slog.Logger,
) (uint64, error) {
	checkpoint, exists, err := persistence.Load(ctx)
	if err != nil {
		return 0, fmt.Errorf("load checkpoint: %w", err)
	}

	if exists {
		startBlock := checkpoint.Number + 1
		logger.InfoContext(
			ctx,
			"resuming from checkpoint",
			"checkpoint", checkpoint.Number,
			"startBlock", startBlock,
		)
		return startBlock, nil
	}

	latestBlock, err := client.GetLatestObservedBlock(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch latest Ethereum block: %w", err)
	}
	logger.InfoContext(
		ctx,
		"starting from latest block",
		"startBlock", latestBlock.Number,
	)

	return latestBlock.Number, nil
}
