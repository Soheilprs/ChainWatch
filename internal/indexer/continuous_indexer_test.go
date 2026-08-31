package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type MockContinuousTransferClient struct {
	*MockTransferBlockClient

	LatestBlock domain.ObservedBlock
	LatestErr   error
	LatestCalls int
}

var _ ContinuousTransferClient = (*MockContinuousTransferClient)(nil)

func (m *MockContinuousTransferClient) GetLatestObservedBlock(
	ctx context.Context,
) (domain.ObservedBlock, error) {
	if err := ctx.Err(); err != nil {
		return domain.ObservedBlock{}, err
	}

	m.LatestCalls++

	if m.LatestErr != nil {
		return domain.ObservedBlock{},
			m.LatestErr
	}

	return m.LatestBlock, nil
}

func TestContinuousIndexerRunCycle(
	t *testing.T,
) {
	blockClient := &MockTransferBlockClient{
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

	client := &MockContinuousTransferClient{
		MockTransferBlockClient: blockClient,
		LatestBlock: domain.ObservedBlock{
			Number: 102,
			Hash:   "0x102",
		},
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	rangeIndexer :=
		NewSequentialIndexer(
			client,
			checkpoints,
		)

	indexer := NewContinuousIndexer(
		client,
		rangeIndexer,
		100,
		time.Second,
		0,
	)

	results, err := indexer.RunCycle(
		context.Background(),
	)

	if err != nil {
		t.Fatalf(
			"expected cycle to succeed, got %v",
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

func TestContinuousIndexerRunCycleResumes(
	t *testing.T,
) {
	blockClient := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			102: {
				Number: 102,
				Hash:   "0x102",
			},
			103: {
				Number: 103,
				Hash:   "0x103",
			},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			102: {
				BlockNumber: 102,
				BlockHash:   "0x102",
			},
			103: {
				BlockNumber: 103,
				BlockHash:   "0x103",
			},
		},
	}

	client := &MockContinuousTransferClient{
		MockTransferBlockClient: blockClient,
		LatestBlock: domain.ObservedBlock{
			Number: 103,
			Hash:   "0x103",
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

	rangeIndexer :=
		NewSequentialIndexer(
			client,
			checkpoints,
		)

	indexer := NewContinuousIndexer(
		client,
		rangeIndexer,
		100,
		time.Second,
		0,
	)

	results, err := indexer.RunCycle(
		ctx,
	)

	if err != nil {
		t.Fatalf(
			"expected cycle to succeed, got %v",
			err,
		)
	}

	if len(results) != 2 {
		t.Fatalf(
			"expected 2 indexed blocks, got %d",
			len(results),
		)
	}

	if len(client.FetchedBlocks) != 2 {
		t.Fatalf(
			"expected 2 fetched blocks, got %d",
			len(client.FetchedBlocks),
		)
	}

	if client.FetchedBlocks[0] != 102 {
		t.Fatalf(
			"expected first fetched block 102, got %d",
			client.FetchedBlocks[0],
		)
	}

	if client.FetchedBlocks[1] != 103 {
		t.Fatalf(
			"expected second fetched block 103, got %d",
			client.FetchedBlocks[1],
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

	if checkpoint.Number != 103 {
		t.Fatalf(
			"expected checkpoint number 103, got %d",
			checkpoint.Number,
		)
	}

	if checkpoint.Hash != "0x103" {
		t.Fatalf(
			"expected checkpoint hash 0x103, got %s",
			checkpoint.Hash,
		)
	}
}

func TestContinuousIndexerStopsOnContextCancellation(
	t *testing.T,
) {
	blockClient := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			100: {
				Number: 100,
				Hash:   "0x100",
			},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			100: {
				BlockNumber: 100,
				BlockHash:   "0x100",
			},
		},
	}

	client := &MockContinuousTransferClient{
		MockTransferBlockClient: blockClient,
		LatestBlock: domain.ObservedBlock{
			Number: 100,
			Hash:   "0x100",
		},
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	rangeIndexer :=
		NewSequentialIndexer(
			client,
			checkpoints,
		)

	indexer := NewContinuousIndexer(
		client,
		rangeIndexer,
		100,
		time.Hour,
		0,
	)

	ctx, cancel :=
		context.WithCancel(
			context.Background(),
		)

	go func() {
		time.Sleep(
			10 * time.Millisecond,
		)

		cancel()
	}()

	err := indexer.Run(
		ctx,
		nil,
	)

	if err != nil {
		t.Fatalf(
			"expected graceful shutdown, got %v",
			err,
		)
	}
}

func TestContinuousIndexerRespectsConfirmationDepth(t *testing.T) {
	blockClient := &MockTransferBlockClient{
		Blocks: map[uint64]domain.ObservedBlock{
			100: {Number: 100, Hash: "0x100"},
			101: {Number: 101, Hash: "0x101"},
			102: {Number: 102, Hash: "0x102"},
		},
		Indexes: map[uint64]domain.BlockTransferIndex{
			100: {BlockNumber: 100, BlockHash: "0x100"},
			101: {BlockNumber: 101, BlockHash: "0x101"},
			102: {BlockNumber: 102, BlockHash: "0x102"},
		},
	}
	client := &MockContinuousTransferClient{
		MockTransferBlockClient: blockClient,
		LatestBlock:             domain.ObservedBlock{Number: 105, Hash: "0x105"},
	}
	persistence := NewMemoryCheckpointStore()
	indexer := NewContinuousIndexer(
		client,
		NewSequentialIndexer(client, persistence),
		100,
		time.Second,
		3,
	)

	results, err := indexer.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("run confirmed cycle: %v", err)
	}
	if len(results) != 3 || results[len(results)-1].BlockNumber != 102 {
		t.Fatalf("unexpected confirmed results: %+v", results)
	}
}
