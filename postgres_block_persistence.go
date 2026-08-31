package main

import (
	"context"
	"fmt"
	"time"

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
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			transactionRollbackTimeout,
		)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	if err := insertBlockTransfers(ctx, tx, index); err != nil {
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
