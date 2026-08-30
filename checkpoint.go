package main

import "context"

type BlockCheckpoint struct {
	Number uint64
	Hash   BlockHash
}

type CheckpointStore interface {
	Load(
		ctx context.Context,
	) (BlockCheckpoint, bool, error)

	Save(
		ctx context.Context,
		checkpoint BlockCheckpoint,
	) error
}

type MemoryCheckpointStore struct {
	checkpoint BlockCheckpoint
	exists     bool
}

var _ CheckpointStore = (*MemoryCheckpointStore)(nil)

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{}
}

func (s *MemoryCheckpointStore) Load(
	ctx context.Context,
) (BlockCheckpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return BlockCheckpoint{}, false, err
	}

	return s.checkpoint, s.exists, nil
}

func (s *MemoryCheckpointStore) Save(
	ctx context.Context,
	checkpoint BlockCheckpoint,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.checkpoint = checkpoint
	s.exists = true

	return nil
}
