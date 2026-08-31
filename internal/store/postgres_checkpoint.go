package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/soheilprs/chainwatch/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCheckpointStore struct {
	pool *pgxpool.Pool
	name string
}

func NewPostgresCheckpointStore(
	pool *pgxpool.Pool,
	name string,
) *PostgresCheckpointStore {
	return &PostgresCheckpointStore{
		pool: pool,
		name: name,
	}
}

func (s *PostgresCheckpointStore) Load(
	ctx context.Context,
) (checkpoint domain.BlockCheckpoint, exists bool, err error) {
	defer domain.ClassifyError(&err, domain.ErrDatabase, "load checkpoint")

	var blockNumber int64
	var blockHash string

	err = s.pool.QueryRow(
		ctx,
		`
		SELECT block_number, block_hash
		FROM checkpoints
		WHERE name = $1
		`,
		s.name,
	).Scan(
		&blockNumber,
		&blockHash,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BlockCheckpoint{}, false, nil
	}

	if err != nil {
		return domain.BlockCheckpoint{}, false, fmt.Errorf(
			"failed to load checkpoint %q: %w",
			s.name,
			err,
		)
	}

	if blockNumber < 0 {
		return domain.BlockCheckpoint{}, false, fmt.Errorf(
			"invalid negative checkpoint block number: %d",
			blockNumber,
		)
	}

	return domain.BlockCheckpoint{
		Number: uint64(blockNumber),
		Hash:   domain.BlockHash(blockHash),
	}, true, nil
}

func (s *PostgresCheckpointStore) Save(
	ctx context.Context,
	checkpoint domain.BlockCheckpoint,
) (err error) {
	defer domain.ClassifyError(&err, domain.ErrDatabase, "save checkpoint")

	err = upsertCheckpoint(ctx, s.pool, s.name, checkpoint)
	if err != nil {
		return fmt.Errorf(
			"failed to save checkpoint %q: %w",
			s.name,
			err,
		)
	}

	return nil
}

type postgresExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func upsertCheckpoint(
	ctx context.Context,
	executor postgresExecutor,
	name string,
	checkpoint domain.BlockCheckpoint,
) error {
	if checkpoint.Number > uint64(^uint64(0)>>1) {
		return fmt.Errorf(
			"checkpoint block number is too large for PostgreSQL BIGINT: %d",
			checkpoint.Number,
		)
	}

	_, err := executor.Exec(
		ctx,
		`
		INSERT INTO checkpoints (
			name,
			block_number,
			block_hash,
			updated_at
		)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (name)
		DO UPDATE SET
			block_number = EXCLUDED.block_number,
			block_hash = EXCLUDED.block_hash,
			updated_at = NOW()
		`,
		name,
		int64(checkpoint.Number),
		string(checkpoint.Hash),
	)

	return err
}
