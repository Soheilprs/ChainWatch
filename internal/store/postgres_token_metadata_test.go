package store

import (
	"context"
	"github.com/soheilprs/chainwatch/internal/domain"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresTokenMetadataStoreSaveAndLoad(
	t *testing.T,
) {
	databaseURL :=
		os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		t.Skip(
			"DATABASE_URL is not set",
		)
	}

	ctx :=
		context.Background()

	pool, err :=
		pgxpool.New(
			ctx,
			databaseURL,
		)

	if err != nil {
		t.Fatalf(
			"failed to create pool: %v",
			err,
		)
	}

	defer pool.Close()

	address :=
		domain.Address(
			"0x1111111111111111111111111111111111111111",
		)

	_, err =
		pool.Exec(
			ctx,
			`
			DELETE FROM token_metadata
			WHERE token_address = $1
			`,
			normalizeMetadataAddress(
				address,
			),
		)

	if err != nil {
		t.Fatalf(
			"failed to clean metadata: %v",
			err,
		)
	}

	defer func() {
		_, _ =
			pool.Exec(
				context.Background(),
				`
				DELETE FROM token_metadata
				WHERE token_address = $1
				`,
				normalizeMetadataAddress(
					address,
				),
			)
	}()

	store :=
		NewPostgresTokenMetadataStore(
			pool,
		)

	expected :=
		domain.TokenMetadata{
			Address: address,

			Name: "Test Token",

			Symbol: "TEST",

			Decimals: 18,
		}

	err =
		store.SaveTokenMetadata(
			ctx,
			expected,
		)

	if err != nil {
		t.Fatalf(
			"failed to save metadata: %v",
			err,
		)
	}

	actual, exists, err :=
		store.LoadTokenMetadata(
			ctx,
			address,
		)

	if err != nil {
		t.Fatalf(
			"failed to load metadata: %v",
			err,
		)
	}

	if !exists {
		t.Fatal(
			"expected metadata to exist",
		)
	}

	if actual.Name != expected.Name {
		t.Fatalf(
			"expected name %s, got %s",
			expected.Name,
			actual.Name,
		)
	}

	if actual.Symbol != expected.Symbol {
		t.Fatalf(
			"expected symbol %s, got %s",
			expected.Symbol,
			actual.Symbol,
		)
	}

	if actual.Decimals !=
		expected.Decimals {

		t.Fatalf(
			"expected decimals %d, got %d",
			expected.Decimals,
			actual.Decimals,
		)
	}
}
