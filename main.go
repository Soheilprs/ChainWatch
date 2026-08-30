package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	rpcURL :=
		os.Getenv("ETH_RPC_URL")

	if rpcURL == "" {
		fmt.Println(
			"ETH_RPC_URL is not set",
		)
		return
	}

	databaseURL :=
		os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		fmt.Println(
			"DATABASE_URL is not set",
		)
		return
	}

	ctx, stop := signal.NotifyContext(
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
		fmt.Println(
			"Failed to create Ethereum client:",
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
		fmt.Println(
			"Failed to create PostgreSQL pool:",
			err,
		)
		return
	}

	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Println(
			"Failed to connect to PostgreSQL:",
			err,
		)
		return
	}

	fmt.Println(
		"Connected to PostgreSQL",
	)

	checkpoints :=
		NewPostgresCheckpointStore(
			pool,
			"ethereum_erc20",
		)

	checkpoint, exists, err :=
		checkpoints.Load(ctx)

	if err != nil {
		fmt.Println(
			"Failed to load checkpoint:",
			err,
		)
		return
	}

	var startBlock uint64

	if exists {
		startBlock =
			checkpoint.Number + 1

		fmt.Println(
			"Resuming after checkpoint:",
			checkpoint.Number,
		)
	} else {
		latestBlock, err :=
			client.GetLatestObservedBlock(ctx)

		if err != nil {
			fmt.Println(
				"Failed to fetch latest Ethereum block:",
				err,
			)
			return
		}

		startBlock =
			latestBlock.Number

		fmt.Println(
			"No checkpoint found",
		)

		fmt.Println(
			"Starting from latest block:",
			startBlock,
		)
	}

	const workerCount = 3

	rangeIndexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			workerCount,
		)

	indexer :=
		NewContinuousIndexer(
			client,
			rangeIndexer,
			startBlock,
			4*time.Second,
		)

	fmt.Println(
		"ChainWatch started",
	)

	fmt.Println(
		"Workers:",
		workerCount,
	)

	fmt.Println(
		"Press Ctrl+C to stop",
	)

	handler := func(
		index BlockTransferIndex,
	) {
		fmt.Printf(
			"Indexed block %d: %d ERC-20 transfers\n",
			index.BlockNumber,
			index.TransferCount(),
		)
	}

	errCh := make(
		chan error,
		1,
	)

	go func() {
		errCh <- indexer.Run(
			ctx,
			handler,
		)
	}()

	err = <-errCh

	if err != nil {
		fmt.Println(
			"Indexer stopped with error:",
			err,
		)
		return
	}

	fmt.Println(
		"ChainWatch stopped cleanly",
	)
}
