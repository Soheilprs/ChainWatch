package indexer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type MockConcurrentTransferBlockClient struct {
	mu sync.Mutex

	Blocks     map[uint64]domain.ObservedBlock
	Indexes    map[uint64]domain.BlockTransferIndex
	Delays     map[uint64]time.Duration
	IndexErrAt uint64

	active    int
	maxActive int
}

var _ TransferBlockClient = (*MockConcurrentTransferBlockClient)(nil)

func (m *MockConcurrentTransferBlockClient) GetObservedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (domain.ObservedBlock, error) {
	if err := ctx.Err(); err != nil {
		return domain.ObservedBlock{}, err
	}

	return m.Blocks[blockNumber], nil
}

func (m *MockConcurrentTransferBlockClient) GetERC20TransfersByBlock(
	ctx context.Context,
	block domain.ObservedBlock,
) (domain.BlockTransferIndex, error) {
	m.mu.Lock()

	m.active++

	if m.active > m.maxActive {
		m.maxActive = m.active
	}

	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.active--
		m.mu.Unlock()
	}()

	delay :=
		m.Delays[block.Number]

	if delay > 0 {
		select {
		case <-time.After(delay):

		case <-ctx.Done():
			return domain.BlockTransferIndex{},
				ctx.Err()
		}
	}

	if block.Number == m.IndexErrAt {
		return domain.BlockTransferIndex{},
			errors.New(
				"mock concurrent indexing failure",
			)
	}

	return m.Indexes[block.Number], nil
}

func (m *MockConcurrentTransferBlockClient) MaxActive() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.maxActive
}

func TestConcurrentRangeIndexerProcessesBlocksConcurrently(
	t *testing.T,
) {
	client :=
		&MockConcurrentTransferBlockClient{
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
			Delays: map[uint64]time.Duration{
				100: 30 * time.Millisecond,
				101: 30 * time.Millisecond,
				102: 30 * time.Millisecond,
			},
		}

	checkpoints :=
		NewMemoryCheckpointStore()

	indexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			3,
		)

	results, err := indexer.IndexRange(
		context.Background(),
		100,
		102,
	)

	if err != nil {
		t.Fatalf(
			"expected concurrent indexing to succeed, got %v",
			err,
		)
	}

	if len(results) != 3 {
		t.Fatalf(
			"expected 3 results, got %d",
			len(results),
		)
	}

	if client.MaxActive() < 2 {
		t.Fatalf(
			"expected concurrent work, max active workers was %d",
			client.MaxActive(),
		)
	}

	if results[0].BlockNumber != 100 {
		t.Fatalf(
			"expected first result block 100, got %d",
			results[0].BlockNumber,
		)
	}

	if results[1].BlockNumber != 101 {
		t.Fatalf(
			"expected second result block 101, got %d",
			results[1].BlockNumber,
		)
	}

	if results[2].BlockNumber != 102 {
		t.Fatalf(
			"expected third result block 102, got %d",
			results[2].BlockNumber,
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

func TestConcurrentRangeIndexerDoesNotAdvancePastFailure(
	t *testing.T,
) {
	client :=
		&MockConcurrentTransferBlockClient{
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
				102: {
					BlockNumber: 102,
					BlockHash:   "0x102",
				},
			},
			Delays: map[uint64]time.Duration{
				100: 20 * time.Millisecond,
				101: 30 * time.Millisecond,
				102: 5 * time.Millisecond,
			},
			IndexErrAt: 101,
		}

	checkpoints :=
		NewMemoryCheckpointStore()

	ctx := context.Background()

	err := checkpoints.Save(
		ctx,
		domain.BlockCheckpoint{
			Number: 99,
			Hash:   "0x99",
		},
	)

	if err != nil {
		t.Fatalf(
			"expected initial checkpoint save to succeed, got %v",
			err,
		)
	}

	indexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			3,
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
			"expected checkpoint to remain at 100, got %d",
			checkpoint.Number,
		)
	}

	if checkpoint.Hash != "0x100" {
		t.Fatalf(
			"expected checkpoint hash 0x100, got %s",
			checkpoint.Hash,
		)
	}
}

func TestConcurrentRangeIndexerRejectsInvalidWorkerCount(
	t *testing.T,
) {
	client :=
		&MockConcurrentTransferBlockClient{}

	checkpoints :=
		NewMemoryCheckpointStore()

	indexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			0, // important: invalid worker count
		)

	_, err := indexer.IndexRange(
		context.Background(),
		100,
		102,
	)

	if !errors.Is(
		err,
		ErrInvalidWorkerCount,
	) {
		t.Fatalf(
			"expected ErrInvalidWorkerCount, got %v",
			err,
		)
	}
}

func TestConcurrentRangeIndexerStopsWhenContextCanceled(
	t *testing.T,
) {
	client :=
		&MockConcurrentTransferBlockClient{
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
				103: {
					Number: 103,
					Hash:   "0x103",
				},
				104: {
					Number: 104,
					Hash:   "0x104",
				},
				105: {
					Number: 105,
					Hash:   "0x105",
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
				103: {
					BlockNumber: 103,
					BlockHash:   "0x103",
				},
				104: {
					BlockNumber: 104,
					BlockHash:   "0x104",
				},
				105: {
					BlockNumber: 105,
					BlockHash:   "0x105",
				},
			},

			Delays: map[uint64]time.Duration{
				100: time.Second,
				101: time.Second,
				102: time.Second,
				103: time.Second,
				104: time.Second,
				105: time.Second,
			},
		}

	checkpoints :=
		NewMemoryCheckpointStore()

	indexer :=
		NewConcurrentRangeIndexer(
			client,
			checkpoints,
			3,
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

	_, err := indexer.IndexRange(
		ctx,
		100,
		105,
	)

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}
