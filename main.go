package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	rpcURL := os.Getenv(
		"ETH_RPC_URL",
	)

	if rpcURL == "" {
		fmt.Println(
			"ETH_RPC_URL is not set",
		)
		return
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	client, err := NewEthereumClient(
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

	latestBlock, err :=
		client.GetLatestObservedBlock(ctx)

	if err != nil {
		fmt.Println(
			"Failed to fetch latest Ethereum block:",
			err,
		)
		return
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	const workerCount = 3

	rangeIndexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			workerCount,
		)

	indexer := NewContinuousIndexer(
		client,
		rangeIndexer,
		latestBlock.Number,
		4*time.Second,
	)

	fmt.Println(
		"ChainWatch started",
	)

	fmt.Println(
		"Starting block:",
		latestBlock.Number,
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
