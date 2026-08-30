package main

import (
	"context"
	"math/big"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresBlockTransferStoreSaveBlock(
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
			"failed to create pool: %v",
			err,
		)
	}

	defer pool.Close()

	const txHash = "0xtest-transfer-save"

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM erc20_transfers
		WHERE transaction_hash = $1
		`,
		txHash,
	)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	index := BlockTransferIndex{
		BlockNumber: 123456,
		BlockHash:   "0xblock123456",

		Transfers: []ERC20Transfer{
			{
				Token: "0xtoken",
				From:  "0xfrom",
				To:    "0xto",

				Value: big.NewInt(1000),

				TransactionHash: txHash,

				LogIndex: 7,
			},
		},
	}

	err =
		store.SaveBlock(
			ctx,
			index,
		)

	if err != nil {
		t.Fatalf(
			"expected save to succeed, got %v",
			err,
		)
	}

	var count int

	err =
		pool.QueryRow(
			ctx,
			`
			SELECT COUNT(*)
			FROM erc20_transfers
			WHERE transaction_hash = $1
			`,
			txHash,
		).Scan(&count)

	if err != nil {
		t.Fatalf(
			"failed to count transfers: %v",
			err,
		)
	}

	if count != 1 {
		t.Fatalf(
			"expected 1 transfer, got %d",
			count,
		)
	}
}

func TestPostgresBlockTransferStoreIsIdempotent(
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
			"failed to create pool: %v",
			err,
		)
	}

	defer pool.Close()

	const txHash = "0xtest-idempotent"

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM erc20_transfers
		WHERE transaction_hash = $1
		`,
		txHash,
	)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	index := BlockTransferIndex{
		BlockNumber: 200,
		BlockHash:   "0x200",

		Transfers: []ERC20Transfer{
			{
				Token: "0xtoken",
				From:  "0xfrom",
				To:    "0xto",

				Value: big.NewInt(500),

				TransactionHash: txHash,

				LogIndex: 3,
			},
		},
	}

	err = store.SaveBlock(
		ctx,
		index,
	)

	if err != nil {
		t.Fatalf(
			"expected first save to succeed, got %v",
			err,
		)
	}

	err = store.SaveBlock(
		ctx,
		index,
	)

	if err != nil {
		t.Fatalf(
			"expected second save to succeed, got %v",
			err,
		)
	}

	var count int

	err =
		pool.QueryRow(
			ctx,
			`
			SELECT COUNT(*)
			FROM erc20_transfers
			WHERE transaction_hash = $1
			`,
			txHash,
		).Scan(&count)

	if err != nil {
		t.Fatalf(
			"failed to count transfers: %v",
			err,
		)
	}

	if count != 1 {
		t.Fatalf(
			"expected exactly 1 transfer after duplicate saves, got %d",
			count,
		)
	}
}

func TestPostgresBlockTransferStoreAllowsMultipleLogsPerTransaction(
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
			"failed to create pool: %v",
			err,
		)
	}

	defer pool.Close()

	const txHash = "0xtest-multiple-logs"

	_, err = pool.Exec(
		ctx,
		`
		DELETE FROM erc20_transfers
		WHERE transaction_hash = $1
		`,
		txHash,
	)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	index := BlockTransferIndex{
		BlockNumber: 300,
		BlockHash:   "0x300",

		Transfers: []ERC20Transfer{
			{
				Token: "0xtoken1",
				From:  "0xfrom1",
				To:    "0xto1",

				Value: big.NewInt(100),

				TransactionHash: txHash,

				LogIndex: 1,
			},
			{
				Token: "0xtoken2",
				From:  "0xfrom2",
				To:    "0xto2",

				Value: big.NewInt(200),

				TransactionHash: txHash,

				LogIndex: 2,
			},
		},
	}

	err =
		store.SaveBlock(
			ctx,
			index,
		)

	if err != nil {
		t.Fatalf(
			"expected save to succeed, got %v",
			err,
		)
	}

	var count int

	err =
		pool.QueryRow(
			ctx,
			`
			SELECT COUNT(*)
			FROM erc20_transfers
			WHERE transaction_hash = $1
			`,
			txHash,
		).Scan(&count)

	if err != nil {
		t.Fatalf(
			"failed to count transfers: %v",
			err,
		)
	}

	if count != 2 {
		t.Fatalf(
			"expected 2 transfers, got %d",
			count,
		)
	}
}
