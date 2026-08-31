package indexer

import (
	"context"
	"math/big"
	"sync"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type CheckpointStore interface {
	Load(
		ctx context.Context,
	) (domain.BlockCheckpoint, bool, error)

	Save(
		ctx context.Context,
		checkpoint domain.BlockCheckpoint,
	) error
}

type MemoryCheckpointStore struct {
	mu sync.RWMutex

	checkpoint domain.BlockCheckpoint
	exists     bool
	indexes    map[uint64]domain.BlockTransferIndex
}

var _ CheckpointStore = (*MemoryCheckpointStore)(nil)
var _ BlockPersistence = (*MemoryCheckpointStore)(nil)
var _ ReorgPersistence = (*MemoryCheckpointStore)(nil)

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{}
}

func (s *MemoryCheckpointStore) Load(
	ctx context.Context,
) (domain.BlockCheckpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.BlockCheckpoint{}, false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.checkpoint, s.exists, nil
}

func (s *MemoryCheckpointStore) Save(
	ctx context.Context,
	checkpoint domain.BlockCheckpoint,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.checkpoint = checkpoint
	s.exists = true

	return nil
}

func (s *MemoryCheckpointStore) SaveIndexedBlock(
	ctx context.Context,
	index domain.BlockTransferIndex,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	storedIndex := cloneBlockTransferIndex(index)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.indexes == nil {
		s.indexes = make(map[uint64]domain.BlockTransferIndex)
	}
	s.indexes[index.BlockNumber] = storedIndex
	s.checkpoint = domain.BlockCheckpoint{
		Number: index.BlockNumber,
		Hash:   index.BlockHash,
	}
	s.exists = true
	return nil
}

func (s *MemoryCheckpointStore) EnsureIndexedBlock(
	ctx context.Context,
	checkpoint domain.BlockCheckpoint,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.indexes == nil {
		s.indexes = make(map[uint64]domain.BlockTransferIndex)
	}
	if existing, exists := s.indexes[checkpoint.Number]; exists && existing.BlockHash != checkpoint.Hash {
		return domain.ErrChainReorg
	}
	s.indexes[checkpoint.Number] = domain.BlockTransferIndex{
		BlockNumber: checkpoint.Number,
		BlockHash:   checkpoint.Hash,
	}
	return nil
}

func (s *MemoryCheckpointStore) LoadIndexedBlockHash(
	ctx context.Context,
	blockNumber uint64,
) (domain.BlockHash, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	index, exists := s.indexes[blockNumber]
	return index.BlockHash, exists, nil
}

func (s *MemoryCheckpointStore) RollbackTo(
	ctx context.Context,
	checkpoint domain.BlockCheckpoint,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for blockNumber := range s.indexes {
		if blockNumber > checkpoint.Number {
			delete(s.indexes, blockNumber)
		}
	}
	s.checkpoint = checkpoint
	s.exists = true
	return nil
}

func cloneBlockTransferIndex(index domain.BlockTransferIndex) domain.BlockTransferIndex {
	clone := domain.BlockTransferIndex{
		BlockNumber: index.BlockNumber,
		BlockHash:   index.BlockHash,
		ParentHash:  index.ParentHash,
		Transfers:   make([]domain.ERC20Transfer, len(index.Transfers)),
	}
	copy(clone.Transfers, index.Transfers)
	for transferIndex := range clone.Transfers {
		if clone.Transfers[transferIndex].Value != nil {
			clone.Transfers[transferIndex].Value = new(big.Int).Set(
				clone.Transfers[transferIndex].Value,
			)
		}
	}
	return clone
}
