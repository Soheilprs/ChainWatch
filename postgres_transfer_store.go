package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresBlockTransferStore struct {
	pool *pgxpool.Pool
}

var _ BlockTransferStore = (*PostgresBlockTransferStore)(nil)

func NewPostgresBlockTransferStore(
	pool *pgxpool.Pool,
) *PostgresBlockTransferStore {
	return &PostgresBlockTransferStore{
		pool: pool,
	}
}

func (s *PostgresBlockTransferStore) SaveBlock(
	ctx context.Context,
	index BlockTransferIndex,
) (err error) {
	defer classifyDomainError(&err, ErrDatabase, "save block transfers")

	tx, err :=
		s.pool.Begin(ctx)

	if err != nil {
		return fmt.Errorf(
			"failed to begin transfer transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := insertBlockTransfers(ctx, tx, index); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf(
			"failed to commit transfer transaction: %w",
			err,
		)
	}

	return nil
}

func insertBlockTransfers(
	ctx context.Context,
	tx pgx.Tx,
	index BlockTransferIndex,
) error {
	for _, transfer := range index.Transfers {
		if transfer.Value == nil {
			return fmt.Errorf(
				"transfer %s log %d has nil value",
				transfer.TransactionHash,
				transfer.LogIndex,
			)
		}

		_, err := tx.Exec(
			ctx,
			`
			INSERT INTO erc20_transfers (
				transaction_hash,
				log_index,
				block_number,
				block_hash,
				token_address,
				from_address,
				to_address,
				value
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (transaction_hash, log_index)
			DO NOTHING
			`,
			string(transfer.TransactionHash),
			int64(transfer.LogIndex),
			int64(index.BlockNumber),
			string(index.BlockHash),
			string(transfer.Token),
			string(transfer.From),
			string(transfer.To),
			transfer.Value.String(),
		)
		if err != nil {
			return fmt.Errorf(
				"save transfer %s log %d: %w",
				transfer.TransactionHash,
				transfer.LogIndex,
				err,
			)
		}
	}
	return nil
}
