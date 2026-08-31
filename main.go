package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger :=
		slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelInfo,
				},
			),
		)

	slog.SetDefault(
		logger,
	)

	metrics :=
		NewMetrics()

	rpcURL :=
		os.Getenv("ETH_RPC_URL")

	if rpcURL == "" {
		logger.Error(
			"ETH_RPC_URL is not set",
		)

		return
	}

	databaseURL :=
		os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		logger.Error(
			"DATABASE_URL is not set",
		)

		return
	}

	ctx, stop :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)

	defer stop()

	client, err :=
		NewEthereumClient(
			ctx,
			rpcURL,
		)

	if err != nil {
		logger.Error(
			"failed to create Ethereum client",
			"error",
			err,
		)

		return
	}

	defer client.Close()

	pool, err :=
		pgxpool.New(
			ctx,
			databaseURL,
		)

	if err != nil {
		logger.Error(
			"failed to create PostgreSQL pool",
			"error",
			err,
		)

		return
	}

	defer pool.Close()

	if err :=
		pool.Ping(ctx); err != nil {

		logger.Error(
			"failed to connect to PostgreSQL",
			"error",
			err,
		)

		return
	}

	logger.Info(
		"connected to PostgreSQL",
	)

	checkpoints :=
		NewPostgresCheckpointStore(
			pool,
			"ethereum_erc20",
		)

	transferStore :=
		NewPostgresBlockTransferStore(
			pool,
		)

	transferReader :=
		NewPostgresTransferReader(
			pool,
		)

	tokenMetadataStore :=
		NewPostgresTokenMetadataStore(
			pool,
		)

	tokenMetadataService :=
		NewTokenMetadataService(
			client,
			tokenMetadataStore,
		)

	checkpoint, exists, err :=
		checkpoints.Load(ctx)

	if err != nil {
		logger.Error(
			"failed to load checkpoint",
			"error",
			err,
		)

		return
	}

	var startBlock uint64

	if exists {
		startBlock =
			checkpoint.Number + 1

		logger.Info(
			"resuming from checkpoint",
			"checkpoint",
			checkpoint.Number,
			"startBlock",
			startBlock,
		)
	} else {
		latestBlock, err :=
			client.GetLatestObservedBlock(
				ctx,
			)

		if err != nil {
			logger.Error(
				"failed to fetch latest Ethereum block",
				"error",
				err,
			)

			return
		}

		startBlock =
			latestBlock.Number

		logger.Info(
			"starting from latest block",
			"startBlock",
			startBlock,
		)
	}

	const workerCount = 3

	rangeIndexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			transferStore,
			workerCount,
		)

	indexer :=
		NewContinuousIndexer(
			client,
			rangeIndexer,
			startBlock,
			4*time.Second,
		)

	api :=
		NewHTTPServerWithObservability(
			transferReader,
			tokenMetadataService,
			logger,
			metrics,
		)

	server :=
		&http.Server{
			Addr: ":8080",

			Handler: api.Handler(),

			ReadHeaderTimeout: 5 * time.Second,
		}

	logger.Info(
		"ChainWatch started",
		"workers",
		workerCount,
		"httpAddress",
		server.Addr,
	)

	errCh :=
		make(
			chan error,
			2,
		)

	go func() {
		errCh <- indexer.Run(
			ctx,
			func(
				index BlockTransferIndex,
			) {
				metrics.RecordIndexedBlock(
					index.TransferCount(),
				)

				logger.Info(
					"indexed block",
					"blockNumber",
					index.BlockNumber,
					"blockHash",
					string(
						index.BlockHash,
					),
					"transfers",
					index.TransferCount(),
				)
			},
		)
	}()

	go func() {
		err :=
			server.ListenAndServe()

		if errors.Is(
			err,
			http.ErrServerClosed,
		) {
			err = nil
		}

		errCh <- err
	}()

	select {
	case <-ctx.Done():

		logger.Info(
			"shutdown signal received",
		)

	case err := <-errCh:
		if err != nil {
			metrics.RecordIndexerError()

			logger.Error(
				"service stopped with error",
				"error",
				err,
			)

			stop()
		}
	}

	shutdownCtx, cancel :=
		context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

	defer cancel()

	if err :=
		server.Shutdown(
			shutdownCtx,
		); err != nil {

		logger.Error(
			"failed to shut down HTTP server",
			"error",
			err,
		)
	}

	logger.Info(
		"ChainWatch stopped cleanly",
	)
}
