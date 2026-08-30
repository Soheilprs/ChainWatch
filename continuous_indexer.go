package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type LatestObservedBlockClient interface {
	GetLatestObservedBlock(
		ctx context.Context,
	) (ObservedBlock, error)
}

type ContinuousTransferClient interface {
	TransferBlockClient
	LatestObservedBlockClient
}

type IndexedBlockHandler func(
	index BlockTransferIndex,
)

type ContinuousIndexer struct {
	client       ContinuousTransferClient
	indexer      *SequentialIndexer
	startBlock   uint64
	pollInterval time.Duration
}

func NewContinuousIndexer(
	client ContinuousTransferClient,
	checkpoints CheckpointStore,
	startBlock uint64,
	pollInterval time.Duration,
) *ContinuousIndexer {
	return &ContinuousIndexer{
		client: client,
		indexer: NewSequentialIndexer(
			client,
			checkpoints,
		),
		startBlock:   startBlock,
		pollInterval: pollInterval,
	}
}

func (c *ContinuousIndexer) RunCycle(
	ctx context.Context,
) ([]BlockTransferIndex, error) {
	latestBlock, err :=
		c.client.GetLatestObservedBlock(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch latest block: %w",
			err,
		)
	}

	indexes, err := c.indexer.IndexRange(
		ctx,
		c.startBlock,
		latestBlock.Number,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to index through block %d: %w",
			latestBlock.Number,
			err,
		)
	}

	return indexes, nil
}

func (c *ContinuousIndexer) Run(
	ctx context.Context,
	handler IndexedBlockHandler,
) error {
	if c.pollInterval <= 0 {
		return errors.New(
			"poll interval must be greater than zero",
		)
	}

	if err := c.runCycleAndHandle(
		ctx,
		handler,
	); err != nil {
		if errors.Is(
			err,
			context.Canceled,
		) {
			return nil
		}

		return err
	}

	ticker := time.NewTicker(
		c.pollInterval,
	)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			err := c.runCycleAndHandle(
				ctx,
				handler,
			)

			if err != nil {
				if errors.Is(
					err,
					context.Canceled,
				) {
					return nil
				}

				return err
			}
		}
	}
}

func (c *ContinuousIndexer) runCycleAndHandle(
	ctx context.Context,
	handler IndexedBlockHandler,
) error {
	indexes, err := c.RunCycle(ctx)

	if err != nil {
		return err
	}

	if handler == nil {
		return nil
	}

	for _, index := range indexes {
		handler(index)
	}

	return nil
}
