package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/soheilprs/chainwatch/internal/domain"
)

var ErrInvalidBlockRange = errors.New(
	"invalid block range",
)

type TransferBlockClient interface {
	GetObservedBlockByNumber(
		ctx context.Context,
		blockNumber uint64,
	) (domain.ObservedBlock, error)

	GetERC20TransfersByBlock(
		ctx context.Context,
		block domain.ObservedBlock,
	) (domain.BlockTransferIndex, error)
}

type SequentialIndexer struct {
	client      TransferBlockClient
	persistence BlockPersistence
}

var _ RangeIndexer = (*SequentialIndexer)(nil)

func NewSequentialIndexer(
	client TransferBlockClient,
	persistence BlockPersistence,
) *SequentialIndexer {
	return &SequentialIndexer{
		client:      client,
		persistence: persistence,
	}
}

func (i *SequentialIndexer) IndexRange(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
) (indexes []domain.BlockTransferIndex, err error) {
	defer domain.ClassifyError(&err, domain.ErrIndexing, "index block range sequentially")

	if startBlock > endBlock {
		return nil, domain.NewDomainError(domain.ErrValidation, "validate block range", ErrInvalidBlockRange)
	}

	nextBlock := startBlock

	checkpoint, exists, err :=
		i.persistence.Load(ctx)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to load checkpoint: %w",
			err,
		)
	}

	if exists &&
		checkpoint.Number >= startBlock {

		if checkpoint.Number >= endBlock {
			return []domain.BlockTransferIndex{},
				nil
		}

		nextBlock =
			checkpoint.Number + 1
	}

	expectedParentHash := domain.BlockHash("")
	if exists && checkpoint.Number+1 == nextBlock {
		expectedParentHash = checkpoint.Hash
	}

	results :=
		make(
			[]domain.BlockTransferIndex,
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

		if err := validateParentHash(
			index.BlockNumber,
			index.ParentHash,
			expectedParentHash,
		); err != nil {
			return nil, err
		}

		err = i.persistence.SaveIndexedBlock(ctx, index)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to persist indexed block %d: %w",
				blockNumber,
				err,
			)
		}
		expectedParentHash = index.BlockHash

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
