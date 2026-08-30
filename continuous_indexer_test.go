package main

import (
	"context"
	"testing"
	"time"
)

type MockContinuousTransferClient struct {
	*MockTransferBlockClient

	LatestBlock ObservedBlock
	LatestErr   error
	LatestCalls int
}

var _ ContinuousTransferClient = (*MockContinuousTransferClient)(nil)

func (m *MockContinuousTransferClient) GetLatestObservedBlock(
	ctx context.Context,
) (ObservedBlock, error) {
	if err := ctx.Err(); err != nil {
		return ObservedBlock{}, err
	}

	m.LatestCalls++

	if m.LatestErr != nil {
		return ObservedBlock{},
			m.LatestErr
	}

	return m.LatestBlock, nil
}

func TestContinuousIndexerRunCycle(
	t *testing.T,
) {
	blockClient := &MockTransferBlockClient{
		Blocks: map[uint64]ObservedBlock{
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
		Indexes: map[uint64]BlockTransferIndex{
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
		LatestBlock: ObservedBlock{
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

	if checkpoint != 102 {
		t.Fatalf(
			"expected checkpoint 102, got %d",
			checkpoint,
		)
	}
}

func TestContinuousIndexerRunCycleResumes(
	t *testing.T,
) {
	blockClient := &MockTransferBlockClient{
		Blocks: map[uint64]ObservedBlock{
			102: {
				Number: 102,
				Hash:   "0x102",
			},
			103: {
				Number: 103,
				Hash:   "0x103",
			},
		},
		Indexes: map[uint64]BlockTransferIndex{
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
		LatestBlock: ObservedBlock{
			Number: 103,
			Hash:   "0x103",
		},
	}

	checkpoints :=
		NewMemoryCheckpointStore()

	ctx := context.Background()

	err := checkpoints.Save(
		ctx,
		101,
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
}

func TestContinuousIndexerStopsOnContextCancellation(
	t *testing.T,
) {
	blockClient := &MockTransferBlockClient{
		Blocks: map[uint64]ObservedBlock{
			100: {
				Number: 100,
				Hash:   "0x100",
			},
		},
		Indexes: map[uint64]BlockTransferIndex{
			100: {
				BlockNumber: 100,
				BlockHash:   "0x100",
			},
		},
	}

	client := &MockContinuousTransferClient{
		MockTransferBlockClient: blockClient,
		LatestBlock: ObservedBlock{
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
