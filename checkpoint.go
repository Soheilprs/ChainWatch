package main

import "context"

type CheckpointStore interface {
	Load(
		ctx context.Context,
	) (uint64, bool, error)

	Save(
		ctx context.Context,
		blockNumber uint64,
	) error
}

type MemoryCheckpointStore struct {
	blockNumber uint64
	exists      bool
}

var _ CheckpointStore = (*MemoryCheckpointStore)(nil)

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{}
}

func (s *MemoryCheckpointStore) Load(
	ctx context.Context,
) (uint64, bool, error) {
	select {
	case <-ctx.Done():
		return 0, false, ctx.Err()
	default:
	}

	return s.blockNumber, s.exists, nil
}

func (s *MemoryCheckpointStore) Save(
	ctx context.Context,
	blockNumber uint64,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.blockNumber = blockNumber
	s.exists = true

	return nil
}
