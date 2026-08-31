package indexer

import (
	"context"
	"testing"

	"github.com/soheilprs/chainwatch/internal/domain"
)

func TestMemoryCheckpointStoreInitiallyEmpty(
	t *testing.T,
) {
	store :=
		NewMemoryCheckpointStore()

	checkpoint, exists, err :=
		store.Load(
			context.Background(),
		)

	if err != nil {
		t.Fatalf(
			"expected load to succeed, got %v",
			err,
		)
	}

	if exists {
		t.Fatal(
			"expected checkpoint store to be empty",
		)
	}

	if checkpoint != (domain.BlockCheckpoint{}) {
		t.Fatalf(
			"expected zero checkpoint, got %+v",
			checkpoint,
		)
	}
}

func TestMemoryCheckpointStoreSaveAndLoad(
	t *testing.T,
) {
	store :=
		NewMemoryCheckpointStore()

	ctx := context.Background()

	expected := domain.BlockCheckpoint{
		Number: 123,
		Hash:   "0xabc",
	}

	err := store.Save(
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
