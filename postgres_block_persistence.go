package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const transactionRollbackTimeout = 5 * time.Second

// PostgresBlockPersistence commits one indexed block and its checkpoint in a
// single PostgreSQL transaction.
type PostgresBlockPersistence struct {
	pool           *pgxpool.Pool
	checkpointName string

	beforeCheckpoint func(context.Context) error
}

var _ BlockPersistence = (*PostgresBlockPersistence)(nil)
var _ ReorgPersistence = (*PostgresBlockPersistence)(nil)

func NewPostgresBlockPersistence(
	pool *pgxpool.Pool,
	checkpointName string,
) *PostgresBlockPersistence {
	return &PostgresBlockPersistence{
		pool:           pool,
		checkpointName: checkpointName,
	}
}

func (s *PostgresBlockPersistence) Load(
	ctx context.Context,
) (BlockCheckpoint, bool, error) {
	return NewPostgresCheckpointStore(s.pool, s.checkpointName).Load(ctx)
}

func (s *PostgresBlockPersistence) SaveIndexedBlock(
	ctx context.Context,
	index BlockTransferIndex,
) (err error) {
	defer classifyDomainError(&err, ErrDatabase, "atomically persist indexed block")

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin indexed block transaction: %w", err)
	}
	defer rollbackPostgresTransaction(ctx, tx)()

	if err := insertBlockTransfers(ctx, tx, index); err != nil {
		return err
	}
	if err := recordIndexedBlock(
		ctx,
		tx,
		index.BlockNumber,
		index.BlockHash,
		index.ParentHash,
	); err != nil {
		return err
	}

	if s.beforeCheckpoint != nil {
		if err := s.beforeCheckpoint(ctx); err != nil {
			return fmt.Errorf("before checkpoint update: %w", err)
		}
	}

	checkpoint := BlockCheckpoint{
		Number: index.BlockNumber,
		Hash:   index.BlockHash,
	}
	if err := upsertCheckpoint(ctx, tx, s.checkpointName, checkpoint); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit indexed block transaction: %w", err)
	}

	return nil
}

func (s *PostgresBlockPersistence) EnsureIndexedBlock(
	ctx context.Context,
	checkpoint BlockCheckpoint,
) (err error) {
	defer classifyDomainError(&err, ErrDatabase, "record checkpoint block history")
	return recordIndexedBlock(ctx, s.pool, checkpoint.Number, checkpoint.Hash, "")
}

func (s *PostgresBlockPersistence) LoadIndexedBlockHash(
	ctx context.Context,
	blockNumber uint64,
) (hash BlockHash, exists bool, err error) {
	defer classifyDomainError(&err, ErrDatabase, "load indexed block hash")

	var storedHash string
	err = s.pool.QueryRow(
		ctx,
		"SELECT block_hash FROM indexed_blocks WHERE block_number = $1",
		int64(blockNumber),
	).Scan(&storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return BlockHash(storedHash), true, nil
}

func (s *PostgresBlockPersistence) RollbackTo(
	ctx context.Context,
	checkpoint BlockCheckpoint,
) (err error) {
	defer classifyDomainError(&err, ErrDatabase, "roll back orphaned indexed blocks")

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reorg rollback transaction: %w", err)
	}
	defer rollbackPostgresTransaction(ctx, tx)()

	if _, err := tx.Exec(
		ctx,
		"DELETE FROM erc20_transfers WHERE block_number > $1",
		int64(checkpoint.Number),
	); err != nil {
		return fmt.Errorf("delete orphaned transfers: %w", err)
	}
	if _, err := tx.Exec(
		ctx,
		"DELETE FROM indexed_blocks WHERE block_number > $1",
		int64(checkpoint.Number),
	); err != nil {
		return fmt.Errorf("delete orphaned block history: %w", err)
	}
	if err := upsertCheckpoint(ctx, tx, s.checkpointName, checkpoint); err != nil {
		return fmt.Errorf("move checkpoint to common ancestor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reorg rollback: %w", err)
	}
	return nil
}

type postgresQueryExecutor interface {
	postgresExecutor
	QueryRow(context.Context, string, ...any) pgx.Row
}

func recordIndexedBlock(
	ctx context.Context,
	executor postgresQueryExecutor,
	blockNumber uint64,
	blockHash BlockHash,
	parentHash BlockHash,
) error {
	var storedHash string
	err := executor.QueryRow(
		ctx,
		`
		INSERT INTO indexed_blocks (block_number, block_hash, parent_hash, indexed_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (block_number)
		DO UPDATE SET
			parent_hash = CASE
				WHEN indexed_blocks.parent_hash = '' THEN EXCLUDED.parent_hash
				ELSE indexed_blocks.parent_hash
			END,
			indexed_at = NOW()
		WHERE indexed_blocks.block_hash = EXCLUDED.block_hash
		RETURNING block_hash
		`,
		int64(blockNumber),
		string(blockHash),
		string(parentHash),
	).Scan(&storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return NewDomainError(
			ErrChainReorg,
			fmt.Sprintf("record indexed block %d", blockNumber),
			errors.New("stored block hash conflicts with canonical block hash"),
		)
	}
	return err
}

func rollbackPostgresTransaction(ctx context.Context, tx pgx.Tx) func() {
	return func() {
		rollbackCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			transactionRollbackTimeout,
		)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}
}
