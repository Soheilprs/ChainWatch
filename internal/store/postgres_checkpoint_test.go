package store

import (
	"context"
	"github.com/soheilprs/chainwatch/internal/domain"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresCheckpointStoreSaveAndLoad(
	t *testing.T,
) {
	databaseURL :=
		os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		t.Skip(
			"DATABASE_URL is not set",
		)
	}

	ctx := context.Background()

	pool, err :=
		pgxpool.New(
			ctx,
			databaseURL,
		)

	if err != nil {
		t.Fatalf(
			"failed to create database pool: %v",
			err,
		)
	}

	defer pool.Close()

	const checkpointName = "test_ethereum_erc20"

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM checkpoints
		WHERE name = $1
		`,
		checkpointName,
	)

	if err != nil {
		t.Fatalf(
			"failed to clean checkpoint: %v",
			err,
		)
	}

	store :=
		NewPostgresCheckpointStore(
			pool,
			checkpointName,
		)

	expected := domain.BlockCheckpoint{
		Number: 123456,
		Hash:   "0xabc123",
	}

	err = store.Save(
		ctx,
		expected,
	)

	if err != nil {
		t.Fatalf(
			"expected save to succeed, got %v",
			err,
		)
	}

	actual, exists, err :=
		store.Load(ctx)

	if err != nil {
		t.Fatalf(
			"expected load to succeed, got %v",
			err,
		)
	}

	if !exists {
		t.Fatal(
			"expected checkpoint to exist",
		)
	}

	if actual != expected {
		t.Fatalf(
			"expected %+v, got %+v",
			expected,
			actual,
		)
	}
}

func TestPostgresCheckpointStoreUpdatesExistingCheckpoint(
	t *testing.T,
) {
	databaseURL :=
		os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		t.Skip(
			"DATABASE_URL is not set",
		)
	}

	ctx := context.Background()

	pool, err :=
		pgxpool.New(
			ctx,
			databaseURL,
		)

	if err != nil {
		t.Fatalf(
			"failed to create database pool: %v",
			err,
		)
	}

	defer pool.Close()

	const checkpointName = "test_checkpoint_update"

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM checkpoints
		WHERE name = $1
		`,
		checkpointName,
	)

	if err != nil {
		t.Fatalf(
			"failed to clean checkpoint: %v",
			err,
		)
	}

	store :=
		NewPostgresCheckpointStore(
			pool,
			checkpointName,
		)

	first := domain.BlockCheckpoint{
		Number: 100,
		Hash:   "0x100",
	}

	err = store.Save(
		ctx,
		first,
	)

	if err != nil {
		t.Fatalf(
			"expected first save to succeed, got %v",
			err,
		)
	}

	second := domain.BlockCheckpoint{
		Number: 101,
		Hash:   "0x101",
	}

	err = store.Save(
		ctx,
		second,
	)

	if err != nil {
		t.Fatalf(
			"expected second save to succeed, got %v",
			err,
		)
	}

	actual, exists, err :=
		store.Load(ctx)

	if err != nil {
		t.Fatalf(
			"expected load to succeed, got %v",
			err,
		)
	}

	if !exists {
		t.Fatal(
			"expected checkpoint to exist",
		)
	}

	if actual.Number != 101 {
		t.Fatalf(
			"expected checkpoint number 101, got %d",
			actual.Number,
		)
	}

	if actual.Hash != "0x101" {
		t.Fatalf(
			"expected checkpoint hash 0x101, got %s",
			actual.Hash,
		)
	}
}

func TestPostgresCheckpointStoreInitiallyEmpty(
	t *testing.T,
) {
	databaseURL :=
		os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		t.Skip(
			"DATABASE_URL is not set",
		)
	}

	ctx := context.Background()

	pool, err :=
		pgxpool.New(
			ctx,
			databaseURL,
		)

	if err != nil {
		t.Fatalf(
			"failed to create database pool: %v",
			err,
		)
	}

	defer pool.Close()

	const checkpointName = "test_empty_checkpoint"

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM checkpoints
		WHERE name = $1
		`,
		checkpointName,
	)

	if err != nil {
		t.Fatalf(
			"failed to clean checkpoint: %v",
			err,
		)
	}

	store :=
		NewPostgresCheckpointStore(
			pool,
			checkpointName,
		)

	checkpoint, exists, err :=
		store.Load(ctx)

	if err != nil {
		t.Fatalf(
			"expected load to succeed, got %v",
			err,
		)
	}

	if exists {
		t.Fatal(
			"expected checkpoint not to exist",
		)
	}

	if checkpoint != (domain.BlockCheckpoint{}) {
		t.Fatalf(
			"expected zero checkpoint, got %+v",
			checkpoint,
		)
	}
}
