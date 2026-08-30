package main

import "context"

type BlockTransferStore interface {
	SaveBlock(
		ctx context.Context,
		index BlockTransferIndex,
	) error
}

type NoopBlockTransferStore struct{}

var _ BlockTransferStore = (*NoopBlockTransferStore)(nil)

func NewNoopBlockTransferStore() *NoopBlockTransferStore {
	return &NoopBlockTransferStore{}
}

func (s *NoopBlockTransferStore) SaveBlock(
	ctx context.Context,
	index BlockTransferIndex,
) error {
	return ctx.Err()
}
