package indexer

import (
	"context"
	"errors"
	"testing"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type MockTransferBlockClient struct {
	Blocks        map[uint64]domain.ObservedBlock
	Indexes       map[uint64]domain.BlockTransferIndex
	FetchErrAt    uint64
	IndexErrAt    uint64
	FetchedBlocks []uint64
	IndexedBlocks []uint64
}

var _ TransferBlockClient = (*MockTransferBlockClient)(nil)

func (m *MockTransferBlockClient) GetObservedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (domain.ObservedBlock, error) {
	if err := ctx.Err(); err != nil {
		return domain.ObservedBlock{}, err
	}

	m.FetchedBlocks = append(
		m.FetchedBlocks,
		blockNumber,
	)

	if blockNumber == m.FetchErrAt {
		return domain.ObservedBlock{},
			errors.New("mock block fetch failure")
	}

	return m.Blocks[blockNumber], nil
}

func (m *MockTransferBlockClient) GetERC20TransfersByBlock(
	ctx context.Context,
	block domain.ObservedBlock,
) (domain.BlockTransferIndex, error) {
	if err := ctx.Err(); err != nil {
		return domain.BlockTransferIndex{}, err
	}

	m.IndexedBlocks = append(
		m.IndexedBlocks,
		block.Number,
	)

	if block.Number == m.IndexErrAt {
		return domain.BlockTransferIndex{},
			errors.New("mock indexing failure")
	}

	return m.Indexes[block.Number], nil
}

func TestSequentialIndexerIndexesRange(
	t *testing.T,
) {
	client := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			100: {
				Number: 100,
				Hash:   "0x100",
			},
			101: {
				Number: 101,
				Hash:   "0x101",
			},
			102: {
				Number: 102,
				Hash:   "0x102",
			},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			100: {
				BlockNumber: 100,
				BlockHash:   "0x100",
			},
			101: {
				BlockNumber: 101,
				BlockHash:   "0x101",
			},
			102: {
				BlockNumber: 102,
				BlockHash:   "0x102",
			},
		},
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	indexer := NewSequentialIndexer(
		client,
		checkpoints,
	)

	results, err := indexer.IndexRange(
		context.Background(),
		100,
		102,
	)

	if err != nil {
		t.Fatalf(
			"expected indexing to succeed, got %v",
			err,
		)
	}

	if len(results) != 3 {
		t.Fatalf(
			"expected 3 indexed blocks, got %d",
			len(results),
		)
	}

	checkpoint, exists, err :=
		checkpoints.Load(
			context.Background(),
		)

	if err != nil {
		t.Fatalf(
			"expected checkpoint load to succeed, got %v",
			err,
		)
	}

	if !exists {
		t.Fatal(
			"expected checkpoint to exist",
		)
	}

	if checkpoint.Number != 102 {
		t.Fatalf(
			"expected checkpoint number 102, got %d",
			checkpoint.Number,
		)
	}

	if checkpoint.Hash != "0x102" {
		t.Fatalf(
			"expected checkpoint hash 0x102, got %s",
			checkpoint.Hash,
		)
	}
}

func TestSequentialIndexerResumesFromCheckpoint(
	t *testing.T,
) {
	client := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			100: {
				Number: 100,
				Hash:   "0x100",
			},
			101: {
				Number: 101,
				Hash:   "0x101",
			},
			102: {
				Number: 102,
				Hash:   "0x102",
			},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			100: {
				BlockNumber: 100,
				BlockHash:   "0x100",
			},
			101: {
				BlockNumber: 101,
				BlockHash:   "0x101",
			},
			102: {
				BlockNumber: 102,
				BlockHash:   "0x102",
			},
		},
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	ctx := context.Background()

	err := checkpoints.Save(
		ctx,
		domain.BlockCheckpoint{
			Number: 101,
			Hash:   "0x101",
		},
	)

	if err != nil {
		t.Fatalf(
			"expected checkpoint save to succeed, got %v",
			err,
		)
	}

	indexer := NewSequentialIndexer(
		client,
		checkpoints,
	)
	results, err := indexer.IndexRange(
		ctx,
		100,
		102,
	)

	if err != nil {
		t.Fatalf(
			"expected indexing to succeed, got %v",
			err,
		)
	}

	if len(results) != 1 {
		t.Fatalf(
			"expected 1 indexed block, got %d",
			len(results),
		)
	}

	if len(client.FetchedBlocks) != 1 {
		t.Fatalf(
			"expected 1 fetched block, got %d",
			len(client.FetchedBlocks),
		)
	}

	if client.FetchedBlocks[0] != 102 {
		t.Fatalf(
			"expected block 102 to be fetched, got %d",
			client.FetchedBlocks[0],
		)
	}

	if len(client.IndexedBlocks) != 1 {
		t.Fatalf(
			"expected 1 indexed block, got %d",
			len(client.IndexedBlocks),
		)
	}

	if client.IndexedBlocks[0] != 102 {
		t.Fatalf(
			"expected block 102 to be indexed, got %d",
			client.IndexedBlocks[0],
		)
	}

	checkpoint, exists, err :=
		checkpoints.Load(ctx)

	if err != nil {
		t.Fatalf(
			"expected checkpoint load to succeed, got %v",
			err,
		)
	}

	if !exists {
		t.Fatal(
			"expected checkpoint to exist",
		)
	}

	if checkpoint.Number != 102 {
		t.Fatalf(
			"expected checkpoint number 102, got %d",
			checkpoint.Number,
		)
	}

	if checkpoint.Hash != "0x102" {
		t.Fatalf(
			"expected checkpoint hash 0x102, got %s",
			checkpoint.Hash,
		)
	}
}

func TestSequentialIndexerDoesNotAdvanceCheckpointOnFailure(
	t *testing.T,
) {
	client := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			101: {
				Number: 101,
				Hash:   "0x101",
			},
			102: {
				Number: 102,
				Hash:   "0x102",
			},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			101: {
				BlockNumber: 101,
				BlockHash:   "0x101",
			},
			102: {
				BlockNumber: 102,
				BlockHash:   "0x102",
			},
		},
		IndexErrAt: 101,
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	ctx := context.Background()

	err := checkpoints.Save(
		ctx,
		domain.BlockCheckpoint{
			Number: 100,
			Hash:   "0x100",
		},
	)

	if err != nil {
		t.Fatalf(
			"expected checkpoint save to succeed, got %v",
			err,
		)
	}

	indexer := NewSequentialIndexer(
		client,
		checkpoints,
	)

	_, err = indexer.IndexRange(
		ctx,
		100,
		102,
	)

	if err == nil {
		t.Fatal(
			"expected indexing to fail",
		)
	}

	if len(client.FetchedBlocks) != 1 {
		t.Fatalf(
			"expected 1 fetched block, got %d",
			len(client.FetchedBlocks),
		)
	}

	if client.FetchedBlocks[0] != 101 {
		t.Fatalf(
			"expected block 101 to be fetched, got %d",
			client.FetchedBlocks[0],
		)
	}

	if len(client.IndexedBlocks) != 1 {
		t.Fatalf(
			"expected 1 indexing attempt, got %d",
			len(client.IndexedBlocks),
		)
	}

	if client.IndexedBlocks[0] != 101 {
		t.Fatalf(
			"expected block 101 to be indexed, got %d",
			client.IndexedBlocks[0],
		)
	}

	checkpoint, exists, loadErr :=
		checkpoints.Load(ctx)

	if loadErr != nil {
		t.Fatalf(
			"expected checkpoint load to succeed, got %v",
			loadErr,
		)
	}

	if !exists {
		t.Fatal(
			"expected checkpoint to exist",
		)
	}

	if checkpoint.Number != 100 {
		t.Fatalf(
			"expected checkpoint number to remain 100, got %d",
			checkpoint.Number,
		)
	}

	if checkpoint.Hash != "0x100" {
		t.Fatalf(
			"expected checkpoint hash to remain 0x100, got %s",
			checkpoint.Hash,
		)
	}
}

func TestSequentialIndexerStopsOnParentHashMismatch(t *testing.T) {
	client := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			101: {Number: 101, Hash: "0x101b", ParentHash: "0xwrong"},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			101: {BlockNumber: 101, BlockHash: "0x101b", ParentHash: "0xwrong"},
		},
	}
	persistence := NewMemoryCheckpointStore()
	ctx := context.Background()
	if err := persistence.Save(ctx, domain.BlockCheckpoint{Number: 100, Hash: "0x100a"}); err != nil {
		t.Fatalf("seed checkpoint: %v", err)
	}

	_, err := NewSequentialIndexer(client, persistence).IndexRange(ctx, 100, 101)
	if !errors.Is(err, domain.ErrChainReorg) {
		t.Fatalf("expected chain reorg error, got %v", err)
	}
	checkpoint, _, loadErr := persistence.Load(ctx)
	if loadErr != nil || checkpoint.Number != 100 || checkpoint.Hash != "0x100a" {
		t.Fatalf("checkpoint advanced after reorg: %+v err=%v", checkpoint, loadErr)
	}
}
