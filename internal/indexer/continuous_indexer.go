package indexer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type LatestObservedBlockClient interface {
	GetLatestObservedBlock(
		ctx context.Context,
	) (domain.ObservedBlock, error)
}

type ContinuousTransferClient interface {
	TransferBlockClient
	LatestObservedBlockClient
}

type IndexedBlockHandler func(
	index domain.BlockTransferIndex,
)

type ContinuousIndexer struct {
	client            LatestObservedBlockClient
	indexer           RangeIndexer
	startBlock        uint64
	pollInterval      time.Duration
	confirmationDepth uint64
}

func NewContinuousIndexer(
	client LatestObservedBlockClient,
	indexer RangeIndexer,
	startBlock uint64,
	pollInterval time.Duration,
	confirmationDepth uint64,
) *ContinuousIndexer {
	return &ContinuousIndexer{
		client:            client,
		indexer:           indexer,
		startBlock:        startBlock,
		pollInterval:      pollInterval,
		confirmationDepth: confirmationDepth,
	}
}

func (c *ContinuousIndexer) RunCycle(
	ctx context.Context,
) ([]domain.BlockTransferIndex, error) {
	latestBlock, err :=
		c.client.GetLatestObservedBlock(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch latest block: %w",
			err,
		)
	}

	if latestBlock.Number < c.confirmationDepth {
		return []domain.BlockTransferIndex{}, nil
	}
	indexThrough := latestBlock.Number - c.confirmationDepth
	if c.startBlock > indexThrough {
		return []domain.BlockTransferIndex{}, nil
	}

	indexes, err := c.indexer.IndexRange(
		ctx,
		c.startBlock,
		indexThrough,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to index through block %d: %w",
			indexThrough,
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
			err :=
				c.runCycleAndHandle(
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
	indexes, err :=
		c.RunCycle(ctx)

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
