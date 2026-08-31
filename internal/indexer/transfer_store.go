package indexer

import (
	"context"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type BlockTransferStore interface {
	SaveBlock(
		ctx context.Context,
		index domain.BlockTransferIndex,
	) error
}

// BlockPersistence is the atomic persistence boundary used by indexers. A
// successful save guarantees that the block's transfers and checkpoint were
// committed together.
type BlockPersistence interface {
	Load(
		ctx context.Context,
	) (domain.BlockCheckpoint, bool, error)

	SaveIndexedBlock(
		ctx context.Context,
		index domain.BlockTransferIndex,
	) error
}

type NoopBlockTransferStore struct{}

var _ BlockTransferStore = (*NoopBlockTransferStore)(nil)

func NewNoopBlockTransferStore() *NoopBlockTransferStore {
	return &NoopBlockTransferStore{}
}

func (s *NoopBlockTransferStore) SaveBlock(
	ctx context.Context,
	index domain.BlockTransferIndex,
) error {
	return ctx.Err()
}
