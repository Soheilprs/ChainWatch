package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresTokenMetadataStore struct {
	pool *pgxpool.Pool
}

var _ TokenMetadataStore = (*PostgresTokenMetadataStore)(nil)

func NewPostgresTokenMetadataStore(
	pool *pgxpool.Pool,
) *PostgresTokenMetadataStore {
	return &PostgresTokenMetadataStore{
		pool: pool,
	}
}

func (s *PostgresTokenMetadataStore) LoadTokenMetadata(
	ctx context.Context,
	address Address,
) (metadata TokenMetadata, exists bool, err error) {
	defer classifyDomainError(&err, ErrDatabase, "load token metadata")

	var storedAddress string
	var name string
	var symbol string
	var decimals int16

	err =
		s.pool.QueryRow(
			ctx,
			`
			SELECT
				token_address,
				name,
				symbol,
				decimals
			FROM token_metadata
			WHERE token_address = $1
			`,
			normalizeMetadataAddress(
				address,
			),
		).Scan(
			&storedAddress,
			&name,
			&symbol,
			&decimals,
		)

	if errors.Is(
		err,
		pgx.ErrNoRows,
	) {
		return TokenMetadata{},
			false,
			nil
	}

	if err != nil {
		return TokenMetadata{},
			false,
			fmt.Errorf(
				"failed to load token metadata: %w",
				err,
			)
	}

	if decimals < 0 ||
		decimals > 255 {

		return TokenMetadata{},
			false,
			fmt.Errorf(
				"invalid token decimals %d",
				decimals,
			)
	}

	return TokenMetadata{
		Address: address,

		Name: name,

		Symbol: symbol,

		Decimals: uint8(decimals),
	}, true, nil
}

func (s *PostgresTokenMetadataStore) SaveTokenMetadata(
	ctx context.Context,
	metadata TokenMetadata,
) (err error) {
	defer classifyDomainError(&err, ErrDatabase, "save token metadata")

	_, err =
		s.pool.Exec(
			ctx,
			`
			INSERT INTO token_metadata (
				token_address,
				name,
				symbol,
				decimals,
				updated_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				NOW()
			)
			ON CONFLICT (token_address)
			DO UPDATE SET
				name = EXCLUDED.name,
				symbol = EXCLUDED.symbol,
				decimals = EXCLUDED.decimals,
				updated_at = NOW()
			`,
			normalizeMetadataAddress(
				metadata.Address,
			),
			metadata.Name,
			metadata.Symbol,
			int16(
				metadata.Decimals,
			),
		)

	if err != nil {
		return fmt.Errorf(
			"failed to save token metadata: %w",
			err,
		)
	}

	return nil
}

func normalizeMetadataAddress(
	address Address,
) string {
	return strings.ToLower(
		string(address),
	)
}
