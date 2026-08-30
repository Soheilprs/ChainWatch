package main

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidBlockRange = errors.New(
	"invalid block range",
)

type TransferBlockClient interface {
	GetObservedBlockByNumber(
		ctx context.Context,
		blockNumber uint64,
	) (ObservedBlock, error)

	GetERC20TransfersByBlock(
		ctx context.Context,
		block ObservedBlock,
	) (BlockTransferIndex, error)
}

type SequentialIndexer struct {
	client      TransferBlockClient
	checkpoints CheckpointStore
}

func NewSequentialIndexer(
	client TransferBlockClient,
	checkpoints CheckpointStore,
) *SequentialIndexer {
	return &SequentialIndexer{
		client:      client,
		checkpoints: checkpoints,
	}
}

func (i *SequentialIndexer) IndexRange(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
) ([]BlockTransferIndex, error) {
	if startBlock > endBlock {
		return nil, ErrInvalidBlockRange
	}

	nextBlock := startBlock

	checkpoint, exists, err :=
		i.checkpoints.Load(ctx)

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

	results := make(
		[]BlockTransferIndex,
		0,
	)

	for blockNumber := nextBlock; blockNumber <= endBlock; blockNumber++ {

		block, err :=
			i.client.GetObservedBlockByNumber(
				ctx,
				blockNumber,
			)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to fetch block %d: %w",
				blockNumber,
				err,
			)
		}

		index, err :=
			i.client.GetERC20TransfersByBlock(
				ctx,
				block,
			)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to index block %d: %w",
				blockNumber,
				err,
			)
		}

		err = i.checkpoints.Save(
			ctx,
			blockNumber,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to save checkpoint for block %d: %w",
				blockNumber,
				err,
			)
		}

		results = append(
			results,
			index,
		)
	}

	return results, nil
}
