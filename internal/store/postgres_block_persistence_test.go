package store

import (
	"context"
	"errors"
	"github.com/soheilprs/chainwatch/internal/domain"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresBlockPersistenceIsAtomicAndIdempotent(t *testing.T) {
	pool := openBlockPersistenceTestPool(t)
	defer pool.Close()

	const checkpointName = "test_atomic_block_success"
	const transactionHash = "0xtest-atomic-block-success"
	cleanupBlockPersistenceData(t, pool, checkpointName, transactionHash, 710001)
	defer cleanupBlockPersistenceData(t, pool, checkpointName, transactionHash, 710001)

	persistence := NewPostgresBlockPersistence(pool, checkpointName)
	index := atomicTestBlockIndex(710001, "0xatomic-success", transactionHash)

	for attempt := 1; attempt <= 2; attempt++ {
		if err := persistence.SaveIndexedBlock(context.Background(), index); err != nil {
			t.Fatalf("save attempt %d: %v", attempt, err)
		}
	}

	var transferCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM erc20_transfers WHERE transaction_hash = $1",
		transactionHash,
	).Scan(&transferCount); err != nil {
		t.Fatalf("count transfers: %v", err)
	}
	if transferCount != 1 {
		t.Fatalf("transfer count = %d, want 1", transferCount)
	}

	checkpoint, exists, err := persistence.Load(context.Background())
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if !exists {
		t.Fatal("expected checkpoint")
	}
	if checkpoint.Number != index.BlockNumber || checkpoint.Hash != index.BlockHash {
		t.Fatalf("checkpoint = %+v, want block %d hash %s", checkpoint, index.BlockNumber, index.BlockHash)
	}
}

func TestPostgresBlockPersistenceRollsBackTransfersWhenCheckpointFails(t *testing.T) {
	pool := openBlockPersistenceTestPool(t)
	defer pool.Close()

	const checkpointName = "test_atomic_block_rollback"
	const transactionHash = "0xtest-atomic-block-rollback"
	cleanupBlockPersistenceData(t, pool, checkpointName, transactionHash, 710002)
	defer cleanupBlockPersistenceData(t, pool, checkpointName, transactionHash, 710002)

	injectedErr := errors.New("injected failure before checkpoint")
	persistence := NewPostgresBlockPersistence(pool, checkpointName)
	persistence.beforeCheckpoint = func(context.Context) error {
		return injectedErr
	}

	err := persistence.SaveIndexedBlock(
		context.Background(),
		atomicTestBlockIndex(710002, "0xatomic-rollback", transactionHash),
	)
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected failure, got %v", err)
	}
	if !errors.Is(err, domain.ErrDatabase) {
		t.Fatalf("expected database classification, got %v", err)
	}

	var transferCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM erc20_transfers WHERE transaction_hash = $1",
		transactionHash,
	).Scan(&transferCount); err != nil {
		t.Fatalf("count transfers after rollback: %v", err)
	}
	if transferCount != 0 {
		t.Fatalf("transfer count after rollback = %d, want 0", transferCount)
	}

	var checkpointCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM checkpoints WHERE name = $1",
		checkpointName,
	).Scan(&checkpointCount); err != nil {
		t.Fatalf("count checkpoints after rollback: %v", err)
	}
	if checkpointCount != 0 {
		t.Fatalf("checkpoint count after rollback = %d, want 0", checkpointCount)
	}

	var historyCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM indexed_blocks WHERE block_number = $1",
		710002,
	).Scan(&historyCount); err != nil {
		t.Fatalf("count block history after rollback: %v", err)
	}
	if historyCount != 0 {
		t.Fatalf("block history count after rollback = %d, want 0", historyCount)
	}
}

func openBlockPersistenceTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	return pool
}

func cleanupBlockPersistenceData(
	t *testing.T,
	pool *pgxpool.Pool,
	checkpointName string,
	transactionHash string,
	blockNumber uint64,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, transferErr := pool.Exec(
		ctx,
		"DELETE FROM erc20_transfers WHERE transaction_hash = $1",
		transactionHash,
	)
	_, checkpointErr := pool.Exec(
		ctx,
		"DELETE FROM checkpoints WHERE name = $1",
		checkpointName,
	)
	_, historyErr := pool.Exec(
		ctx,
		"DELETE FROM indexed_blocks WHERE block_number = $1",
		int64(blockNumber),
	)
	if err := errors.Join(transferErr, checkpointErr, historyErr); err != nil {
		t.Errorf("clean atomic persistence test data: %v", err)
	}
}

func atomicTestBlockIndex(
	blockNumber uint64,
	blockHash domain.BlockHash,
	transactionHash string,
) domain.BlockTransferIndex {
	return domain.BlockTransferIndex{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		Transfers: []domain.ERC20Transfer{
			{
				Token:           "0xatomic-token",
				From:            "0xatomic-from",
				To:              "0xatomic-to",
				Value:           big.NewInt(1000),
				TransactionHash: domain.TransactionHash(transactionHash),
				LogIndex:        1,
			},
		},
	}
}
