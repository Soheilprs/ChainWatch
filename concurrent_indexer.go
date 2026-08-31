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
	persistence BlockPersistence
	workerCount int
}

var _ RangeIndexer = (*ConcurrentRangeIndexer)(nil)

func NewConcurrentRangeIndexer(
	client TransferBlockClient,
	persistence BlockPersistence,
	workerCount int,
) *ConcurrentRangeIndexer {
	return &ConcurrentRangeIndexer{
		client:      client,
		persistence: persistence,
		workerCount: workerCount,
	}
}

func (c *ConcurrentRangeIndexer) IndexRange(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
) (indexes []BlockTransferIndex, err error) {
	defer classifyDomainError(&err, ErrIndexing, "index block range concurrently")

	if startBlock > endBlock {
		return nil, NewDomainError(ErrValidation, "validate block range", ErrInvalidBlockRange)
	}

	if c.workerCount <= 0 {
		return nil, NewDomainError(ErrValidation, "validate worker count", ErrInvalidWorkerCount)
	}

	nextBlock := startBlock

	checkpoint, exists, err :=
		c.persistence.Load(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to load checkpoint: %w",
			err,
		)
	}

	if exists &&
		checkpoint.Number >= startBlock {

		if checkpoint.Number >= endBlock {
			return []BlockTransferIndex{},
				nil
		}

		nextBlock =
			checkpoint.Number + 1
	}

	expectedParentHash := BlockHash("")
	if exists && checkpoint.Number+1 == nextBlock {
		expectedParentHash = checkpoint.Hash
	}

	workerCtx, cancel :=
		context.WithCancel(ctx)

	defer cancel()

	jobs :=
		make(
			chan uint64,
		)

	results :=
		make(
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

	pending :=
		make(
			map[uint64]concurrentBlockResult,
		)

	orderedResults :=
		make(
			[]BlockTransferIndex,
			0,
		)

	nextCommit := nextBlock

	for result := range results {

		pending[result.blockNumber] = result

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

			if err := validateParentHash(
				nextResult.index.BlockNumber,
				nextResult.index.ParentHash,
				expectedParentHash,
			); err != nil {
				cancel()
				return nil, err
			}

			err := c.persistence.SaveIndexedBlock(ctx, nextResult.index)
			if err != nil {
				cancel()

				return nil, fmt.Errorf(
					"failed to persist indexed block %d: %w",
					nextCommit,
					err,
				)
			}
			expectedParentHash = nextResult.index.BlockHash

			orderedResults =
				append(
					orderedResults,
					nextResult.index,
				)

			delete(
				pending,
				nextCommit,
			)

			if nextCommit ==
				endBlock {

				return orderedResults,
					nil
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
