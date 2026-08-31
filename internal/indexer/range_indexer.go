package indexer

import (
	"context"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type RangeIndexer interface {
	IndexRange(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
	) ([]domain.BlockTransferIndex, error)
}
