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
	transfers   BlockTransferStore
}

var _ RangeIndexer = (*SequentialIndexer)(nil)

func NewSequentialIndexer(
	client TransferBlockClient,
	checkpoints CheckpointStore,
	transfers BlockTransferStore,
) *SequentialIndexer {
	return &SequentialIndexer{
		client:      client,
		checkpoints: checkpoints,
		transfers:   transfers,
	}
}

func (i *SequentialIndexer) IndexRange(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
) (indexes []BlockTransferIndex, err error) {
	defer classifyDomainError(&err, ErrIndexing, "index block range sequentially")

	if startBlock > endBlock {
		return nil, NewDomainError(ErrValidation, "validate block range", ErrInvalidBlockRange)
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

	if exists &&
		checkpoint.Number >= startBlock {

		if checkpoint.Number >= endBlock {
			return []BlockTransferIndex{},
				nil
		}

		nextBlock =
			checkpoint.Number + 1
	}

	results :=
		make(
			[]BlockTransferIndex,
			0,
		)

	for blockNumber := nextBlock; ; blockNumber++ {
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

		err =
			i.transfers.SaveBlock(
				ctx,
				index,
			)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to save transfers for block %d: %w",
				blockNumber,
				err,
			)
		}

		err =
			i.checkpoints.Save(
				ctx,
				BlockCheckpoint{
					Number: block.Number,
					Hash:   block.Hash,
				},
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

		if blockNumber == endBlock {
			break
		}
	}

	return results, nil
}
