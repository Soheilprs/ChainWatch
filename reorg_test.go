package main

import (
	"context"
	"errors"
	"testing"
)

func TestReorgManagerRollsBackToCommonAncestor(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()
	for _, index := range []BlockTransferIndex{
		{BlockNumber: 100, BlockHash: "0x100a"},
		{BlockNumber: 101, BlockHash: "0x101a", ParentHash: "0x100a"},
		{BlockNumber: 102, BlockHash: "0x102a", ParentHash: "0x101a"},
	} {
		if err := store.SaveIndexedBlock(ctx, index); err != nil {
			t.Fatalf("seed block %d: %v", index.BlockNumber, err)
		}
	}

	client := &MockTransferBlockClient{Blocks: map[uint64]ObservedBlock{
		100: {Number: 100, Hash: "0x100a"},
		101: {Number: 101, Hash: "0x101b"},
		102: {Number: 102, Hash: "0x102b"},
	}}

	result, err := NewReorgManager(client, store, 10).Reconcile(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !result.ReorgDetected {
		t.Fatal("expected reorg detection")
	}
	if result.PreviousCheckpoint.Number != 102 || result.Checkpoint.Number != 100 {
		t.Fatalf("unexpected reorg result: %+v", result)
	}

	checkpoint, exists, err := store.Load(ctx)
	if err != nil || !exists {
		t.Fatalf("load checkpoint: exists=%v err=%v", exists, err)
	}
	if checkpoint.Number != 100 || checkpoint.Hash != "0x100a" {
		t.Fatalf("checkpoint after rollback = %+v", checkpoint)
	}
	if _, exists, err := store.LoadIndexedBlockHash(ctx, 101); err != nil || exists {
		t.Fatalf("orphan block 101 remains: exists=%v err=%v", exists, err)
	}
}

func TestReorgManagerRejectsReorgBeyondMaximumDepth(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()
	for _, index := range []BlockTransferIndex{
		{BlockNumber: 100, BlockHash: "0x100a"},
		{BlockNumber: 101, BlockHash: "0x101a"},
		{BlockNumber: 102, BlockHash: "0x102a"},
	} {
		if err := store.SaveIndexedBlock(ctx, index); err != nil {
			t.Fatalf("seed block %d: %v", index.BlockNumber, err)
		}
	}

	client := &MockTransferBlockClient{Blocks: map[uint64]ObservedBlock{
		101: {Number: 101, Hash: "0x101b"},
		102: {Number: 102, Hash: "0x102b"},
	}}
	_, err := NewReorgManager(client, store, 1).Reconcile(ctx)
	if !errors.Is(err, ErrReorgTooDeep) || !errors.Is(err, ErrChainReorg) {
		t.Fatalf("expected bounded reorg error, got %v", err)
	}

	checkpoint, _, loadErr := store.Load(ctx)
	if loadErr != nil || checkpoint.Number != 102 {
		t.Fatalf("checkpoint changed after rejected rollback: %+v err=%v", checkpoint, loadErr)
	}
}

func TestReorgManagerFailsSafeWithoutHistory(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()
	if err := store.Save(ctx, BlockCheckpoint{Number: 102, Hash: "0x102a"}); err != nil {
		t.Fatalf("seed checkpoint: %v", err)
	}
	client := &MockTransferBlockClient{Blocks: map[uint64]ObservedBlock{
		102: {Number: 102, Hash: "0x102b"},
	}}

	_, err := NewReorgManager(client, store, 10).Reconcile(ctx)
	if !errors.Is(err, ErrReorgHistoryUnavailable) || !errors.Is(err, ErrChainReorg) {
		t.Fatalf("expected missing history error, got %v", err)
	}
}

func TestReorgManagerRecordsCanonicalLegacyCheckpoint(t *testing.T) {
	store := NewMemoryCheckpointStore()
	ctx := context.Background()
	checkpoint := BlockCheckpoint{Number: 102, Hash: "0x102a"}
	if err := store.Save(ctx, checkpoint); err != nil {
		t.Fatalf("seed checkpoint: %v", err)
	}
	client := &MockTransferBlockClient{Blocks: map[uint64]ObservedBlock{
		102: {Number: 102, Hash: "0x102a"},
	}}

	result, err := NewReorgManager(client, store, 10).Reconcile(ctx)
	if err != nil || result.ReorgDetected {
		t.Fatalf("unexpected reconciliation result: %+v err=%v", result, err)
	}
	hash, exists, err := store.LoadIndexedBlockHash(ctx, 102)
	if err != nil || !exists || hash != checkpoint.Hash {
		t.Fatalf("legacy checkpoint history was not recorded: hash=%s exists=%v err=%v", hash, exists, err)
	}
}

func TestValidateParentHashDetectsLiveReorg(t *testing.T) {
	err := validateParentHash(103, "0xother-parent", "0xexpected-parent")
	if !errors.Is(err, ErrChainReorg) {
		t.Fatalf("expected chain reorg error, got %v", err)
	}
}
