package indexer

import (
	"context"
	"errors"
	"fmt"

	"github.com/soheilprs/chainwatch/internal/domain"
)

var (
	ErrReorgTooDeep            = errors.New("reorganization exceeds maximum rollback depth")
	ErrReorgHistoryUnavailable = errors.New("indexed block history is unavailable")
)

type ReorgPersistence interface {
	BlockPersistence

	EnsureIndexedBlock(
		ctx context.Context,
		checkpoint domain.BlockCheckpoint,
	) error

	LoadIndexedBlockHash(
		ctx context.Context,
		blockNumber uint64,
	) (domain.BlockHash, bool, error)

	RollbackTo(
		ctx context.Context,
		checkpoint domain.BlockCheckpoint,
	) error
}

type CanonicalBlockClient interface {
	GetObservedBlockByNumber(
		ctx context.Context,
		blockNumber uint64,
	) (domain.ObservedBlock, error)
}

type ReorgResult struct {
	Checkpoint         domain.BlockCheckpoint
	PreviousCheckpoint domain.BlockCheckpoint
	Exists             bool
	ReorgDetected      bool
}

type ReorgManager struct {
	client   CanonicalBlockClient
	store    ReorgPersistence
	maxDepth uint64
}

func NewReorgManager(
	client CanonicalBlockClient,
	store ReorgPersistence,
	maxDepth uint64,
) *ReorgManager {
	return &ReorgManager{
		client:   client,
		store:    store,
		maxDepth: maxDepth,
	}
}

// Reconcile verifies the saved checkpoint against the canonical chain. On a
// mismatch it finds the nearest recorded common ancestor within maxDepth and
// atomically removes all later indexed data.
func (m *ReorgManager) Reconcile(ctx context.Context) (ReorgResult, error) {
	if m.maxDepth == 0 {
		return ReorgResult{}, domain.NewDomainError(
			domain.ErrValidation,
			"validate maximum reorg depth",
			errors.New("maximum reorg depth must be greater than zero"),
		)
	}

	checkpoint, exists, err := m.store.Load(ctx)
	if err != nil {
		return ReorgResult{}, fmt.Errorf("load checkpoint for reorg verification: %w", err)
	}
	if !exists {
		return ReorgResult{}, nil
	}

	canonical, err := m.client.GetObservedBlockByNumber(ctx, checkpoint.Number)
	if err != nil {
		return ReorgResult{}, fmt.Errorf(
			"fetch canonical block %d for reorg verification: %w",
			checkpoint.Number,
			err,
		)
	}
	if canonical.Hash == checkpoint.Hash {
		if err := m.store.EnsureIndexedBlock(ctx, checkpoint); err != nil {
			return ReorgResult{}, fmt.Errorf("record checkpoint history: %w", err)
		}
		return ReorgResult{Checkpoint: checkpoint, Exists: true}, nil
	}

	height := checkpoint.Number
	for depth := uint64(1); depth <= m.maxDepth && height > 0; depth++ {
		height--

		storedHash, found, err := m.store.LoadIndexedBlockHash(ctx, height)
		if err != nil {
			return ReorgResult{}, fmt.Errorf("load indexed block %d: %w", height, err)
		}
		if !found {
			return ReorgResult{}, domain.NewDomainError(
				ErrReorgHistoryUnavailable,
				fmt.Sprintf("find common ancestor at block %d", height),
				domain.ErrChainReorg,
			)
		}

		canonical, err := m.client.GetObservedBlockByNumber(ctx, height)
		if err != nil {
			return ReorgResult{}, fmt.Errorf("fetch canonical block %d: %w", height, err)
		}
		if canonical.Hash != storedHash {
			continue
		}

		ancestor := domain.BlockCheckpoint{Number: height, Hash: storedHash}
		if err := m.store.RollbackTo(ctx, ancestor); err != nil {
			return ReorgResult{}, fmt.Errorf("roll back to common ancestor %d: %w", height, err)
		}
		return ReorgResult{
			Checkpoint:         ancestor,
			PreviousCheckpoint: checkpoint,
			Exists:             true,
			ReorgDetected:      true,
		}, nil
	}

	return ReorgResult{}, domain.NewDomainError(
		ErrReorgTooDeep,
		fmt.Sprintf("find common ancestor for checkpoint %d", checkpoint.Number),
		domain.ErrChainReorg,
	)
}

func validateParentHash(
	blockNumber uint64,
	parentHash domain.BlockHash,
	expectedParentHash domain.BlockHash,
) error {
	if parentHash == "" || expectedParentHash == "" || parentHash == expectedParentHash {
		return nil
	}
	return domain.NewDomainError(
		domain.ErrChainReorg,
		fmt.Sprintf("validate parent hash for block %d", blockNumber),
		fmt.Errorf("parent hash %s does not match checkpoint hash %s", parentHash, expectedParentHash),
	)
}
