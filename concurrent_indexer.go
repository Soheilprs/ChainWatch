package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrInvalidWorkerCount = errors.New(
	"worker count must be greater than zero",
)

type concurrentBlockResult struct {
	blockNumber uint64
	index       BlockTransferIndex
	err         error
}

type ConcurrentRangeIndexer struct {
	client      TransferBlockClient
	checkpoints CheckpointStore
	workerCount int
}

var _ RangeIndexer = (*ConcurrentRangeIndexer)(nil)

func NewConcurrentRangeIndexer(
	client TransferBlockClient,
	checkpoints CheckpointStore,
	workerCount int,
) *ConcurrentRangeIndexer {
	return &ConcurrentRangeIndexer{
		client:      client,
		checkpoints: checkpoints,
		workerCount: workerCount,
	}
}

func (c *ConcurrentRangeIndexer) IndexRange(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
) ([]BlockTransferIndex, error) {
	if startBlock > endBlock {
		return nil, ErrInvalidBlockRange
	}

	if c.workerCount <= 0 {
		return nil, ErrInvalidWorkerCount
	}

	nextBlock := startBlock

	checkpoint, exists, err :=
		c.checkpoints.Load(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to load checkpoint: %w",
			err,
		)
	}

	if exists && checkpoint >= startBlock {
		if checkpoint >= endBlock {
			return []BlockTransferIndex{}, nil
		}

		nextBlock = checkpoint + 1
	}

	workerCtx, cancel :=
		context.WithCancel(ctx)

	defer cancel()

	jobs := make(
		chan uint64,
	)

	results := make(
		chan concurrentBlockResult,
		c.workerCount,
	)

	var workers sync.WaitGroup

	for workerID := 0; workerID < c.workerCount; workerID++ {

		workers.Add(1)

		go func() {
			defer workers.Done()

			for blockNumber := range jobs {
				result :=
					c.processBlock(
						workerCtx,
						blockNumber,
					)

				select {
				case results <- result:

				case <-workerCtx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)

		for blockNumber := nextBlock; ; blockNumber++ {
			select {
			case jobs <- blockNumber:

			case <-workerCtx.Done():
				return
			}

			if blockNumber == endBlock {
				return
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	pending := make(
		map[uint64]concurrentBlockResult,
	)

	orderedResults := make(
		[]BlockTransferIndex,
		0,
	)

	nextCommit := nextBlock

	for result := range results {
		pending[result.blockNumber] =
			result

		for {
			nextResult, found :=
				pending[nextCommit]

			if !found {
				break
			}

			if nextResult.err != nil {
				cancel()

				return nil,
					nextResult.err
			}

			err := c.checkpoints.Save(
				ctx,
				nextCommit,
			)

			if err != nil {
				cancel()

				return nil, fmt.Errorf(
					"failed to save checkpoint for block %d: %w",
					nextCommit,
					err,
				)
			}

			orderedResults = append(
				orderedResults,
				nextResult.index,
			)

			delete(
				pending,
				nextCommit,
			)

			if nextCommit == endBlock {
				return orderedResults, nil
			}

			nextCommit++
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return orderedResults, nil
}

func (c *ConcurrentRangeIndexer) processBlock(
	ctx context.Context,
	blockNumber uint64,
) concurrentBlockResult {
	block, err :=
		c.client.GetObservedBlockByNumber(
			ctx,
			blockNumber,
		)

	if err != nil {
		return concurrentBlockResult{
			blockNumber: blockNumber,
			err: fmt.Errorf(
				"failed to fetch block %d: %w",
				blockNumber,
				err,
			),
		}
	}

	index, err :=
		c.client.GetERC20TransfersByBlock(
			ctx,
			block,
		)

	if err != nil {
		return concurrentBlockResult{
			blockNumber: blockNumber,
			err: fmt.Errorf(
				"failed to index block %d: %w",
				blockNumber,
				err,
			),
		}
	}

	return concurrentBlockResult{
		blockNumber: blockNumber,
		index:       index,
	}
}
