package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/soheilprs/chainwatch/internal/domain"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresBlockPersistenceRollsBackOrphanedBlocksAtomically(t *testing.T) {
	pool := openBlockPersistenceTestPool(t)
	defer pool.Close()

	const checkpointName = "test_reorg_atomic_rollback"
	const firstBlock = uint64(720001)
	const lastBlock = uint64(720003)
	cleanupPostgresReorgData(t, pool, checkpointName, firstBlock, lastBlock)
	defer cleanupPostgresReorgData(t, pool, checkpointName, firstBlock, lastBlock)

	persistence := NewPostgresBlockPersistence(pool, checkpointName)
	parentHash := domain.BlockHash("0xreorg-parent")
	for blockNumber := firstBlock; blockNumber <= lastBlock; blockNumber++ {
		blockHash := domain.BlockHash(fmt.Sprintf("0xreorg-%d", blockNumber))
		index := atomicTestBlockIndex(
			blockNumber,
			blockHash,
			fmt.Sprintf("0xtest-reorg-%d", blockNumber),
		)
		index.ParentHash = parentHash
		if err := persistence.SaveIndexedBlock(context.Background(), index); err != nil {
			t.Fatalf("save block %d: %v", blockNumber, err)
		}
		parentHash = blockHash
	}

	ancestor := domain.BlockCheckpoint{
		Number: firstBlock,
		Hash:   domain.BlockHash(fmt.Sprintf("0xreorg-%d", firstBlock)),
	}
	if err := persistence.RollbackTo(context.Background(), ancestor); err != nil {
		t.Fatalf("rollback to common ancestor: %v", err)
	}

	checkpoint, exists, err := persistence.Load(context.Background())
	if err != nil || !exists {
		t.Fatalf("load checkpoint: exists=%v err=%v", exists, err)
	}
	if checkpoint != ancestor {
		t.Fatalf("checkpoint = %+v, want %+v", checkpoint, ancestor)
	}

	var transferCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM erc20_transfers WHERE block_number > $1 AND block_number <= $2",
		int64(firstBlock),
		int64(lastBlock),
	).Scan(&transferCount); err != nil {
		t.Fatalf("count orphaned transfers: %v", err)
	}
	if transferCount != 0 {
		t.Fatalf("orphaned transfer count = %d, want 0", transferCount)
	}

	var historyCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM indexed_blocks WHERE block_number > $1 AND block_number <= $2",
		int64(firstBlock),
		int64(lastBlock),
	).Scan(&historyCount); err != nil {
		t.Fatalf("count orphaned block history: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("orphaned history count = %d, want 0", historyCount)
	}
}

func cleanupPostgresReorgData(
	t *testing.T,
	pool *pgxpool.Pool,
	checkpointName string,
	firstBlock uint64,
	lastBlock uint64,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, transferErr := pool.Exec(
		ctx,
		"DELETE FROM erc20_transfers WHERE block_number >= $1 AND block_number <= $2",
		int64(firstBlock),
		int64(lastBlock),
	)
	_, checkpointErr := pool.Exec(
		ctx,
		"DELETE FROM checkpoints WHERE name = $1",
		checkpointName,
	)
	_, historyErr := pool.Exec(
		ctx,
		"DELETE FROM indexed_blocks WHERE block_number >= $1 AND block_number <= $2",
		int64(firstBlock),
		int64(lastBlock),
	)
	if err := errors.Join(transferErr, checkpointErr, historyErr); err != nil {
		t.Errorf("clean reorg test data: %v", err)
	}
}
