package store

import (
	"context"
	"github.com/soheilprs/chainwatch/internal/domain"
	"math/big"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresTransferReaderPaginatesWithoutDuplicates(
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

	const token = "0xtest-pagination-token"

	_, err =
		pool.Exec(
			ctx,
			`
		DELETE FROM erc20_transfers
		WHERE token_address = $1
		`,
			token,
		)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	defer func() {
		_, _ =
			pool.Exec(
				context.Background(),
				`
			DELETE FROM erc20_transfers
			WHERE token_address = $1
			`,
				token,
			)
	}()

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	index :=
		domain.BlockTransferIndex{
			BlockNumber: 900000001,
			BlockHash:   "0xtest-block",

			Transfers: []domain.ERC20Transfer{
				{
					Token: token,

					From: "0xfrom",

					To: "0xto",

					Value: big.NewInt(1),

					TransactionHash: "0xtest-pagination-1",

					LogIndex: 30,
				},
				{
					Token: token,

					From: "0xfrom",

					To: "0xto",

					Value: big.NewInt(2),

					TransactionHash: "0xtest-pagination-2",

					LogIndex: 20,
				},
				{
					Token: token,

					From: "0xfrom",

					To: "0xto",

					Value: big.NewInt(3),

					TransactionHash: "0xtest-pagination-3",

					LogIndex: 10,
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
			"failed to save test transfers: %v",
			err,
		)
	}

	reader :=
		NewPostgresTransferReader(
			pool,
		)

	tokenAddress :=
		domain.Address(token)

	firstPage, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 2,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected first page to succeed, got %v",
			err,
		)
	}

	if len(firstPage.Transfers) != 2 {
		t.Fatalf(
			"expected 2 transfers on first page, got %d",
			len(firstPage.Transfers),
		)
	}

	if firstPage.NextCursor == nil {
		t.Fatal(
			"expected next cursor",
		)
	}

	if firstPage.Transfers[0].
		LogIndex != 30 {

		t.Fatalf(
			"expected first log index 30, got %d",
			firstPage.Transfers[0].
				LogIndex,
		)
	}

	if firstPage.Transfers[1].
		LogIndex != 20 {

		t.Fatalf(
			"expected second log index 20, got %d",
			firstPage.Transfers[1].
				LogIndex,
		)
	}

	secondPage, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 2,

				Cursor: firstPage.
					NextCursor,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected second page to succeed, got %v",
			err,
		)
	}

	if len(secondPage.Transfers) != 1 {
		t.Fatalf(
			"expected 1 transfer on second page, got %d",
			len(secondPage.Transfers),
		)
	}

	if secondPage.Transfers[0].
		LogIndex != 10 {

		t.Fatalf(
			"expected log index 10, got %d",
			secondPage.Transfers[0].
				LogIndex,
		)
	}

	if secondPage.NextCursor != nil {
		t.Fatal(
			"expected no next cursor on final page",
		)
	}

	if firstPage.Transfers[1].
		TransactionHash ==
		secondPage.Transfers[0].
			TransactionHash {

		t.Fatal(
			"expected pages not to overlap",
		)
	}
}

func TestPostgresTransferReaderPaginatesAcrossBlocks(
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

	const token = "0xtest-pagination-across-blocks"

	_, err =
		pool.Exec(
			ctx,
			`
			DELETE FROM erc20_transfers
			WHERE token_address = $1
			`,
			token,
		)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	defer func() {
		_, _ =
			pool.Exec(
				context.Background(),
				`
				DELETE FROM erc20_transfers
				WHERE token_address = $1
				`,
				token,
			)
	}()

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	err =
		store.SaveBlock(
			ctx,
			domain.BlockTransferIndex{
				BlockNumber: 900000200,

				BlockHash: "0xtest-block-200",

				Transfers: []domain.ERC20Transfer{
					{
						Token: token,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(1),

						TransactionHash: "0xtest-cross-block-1",

						LogIndex: 20,
					},
					{
						Token: token,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(2),

						TransactionHash: "0xtest-cross-block-2",

						LogIndex: 10,
					},
				},
			},
		)

	if err != nil {
		t.Fatalf(
			"failed to save block 200: %v",
			err,
		)
	}

	err =
		store.SaveBlock(
			ctx,
			domain.BlockTransferIndex{
				BlockNumber: 900000199,

				BlockHash: "0xtest-block-199",

				Transfers: []domain.ERC20Transfer{
					{
						Token: token,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(3),

						TransactionHash: "0xtest-cross-block-3",

						LogIndex: 100,
					},
					{
						Token: token,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(4),

						TransactionHash: "0xtest-cross-block-4",

						LogIndex: 50,
					},
				},
			},
		)

	if err != nil {
		t.Fatalf(
			"failed to save block 199: %v",
			err,
		)
	}

	reader :=
		NewPostgresTransferReader(
			pool,
		)

	tokenAddress :=
		domain.Address(token)

	firstPage, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 3,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected first page to succeed, got %v",
			err,
		)
	}

	if len(firstPage.Transfers) != 3 {
		t.Fatalf(
			"expected 3 transfers, got %d",
			len(firstPage.Transfers),
		)
	}

	expectedFirst :=
		[]struct {
			block uint64
			log   uint
		}{
			{
				block: 900000200,
				log:   20,
			},
			{
				block: 900000200,
				log:   10,
			},
			{
				block: 900000199,
				log:   100,
			},
		}

	for index, expected := range expectedFirst {

		actual :=
			firstPage.Transfers[index]

		if actual.BlockNumber !=
			expected.block {

			t.Fatalf(
				"expected block %d at index %d, got %d",
				expected.block,
				index,
				actual.BlockNumber,
			)
		}

		if actual.LogIndex !=
			expected.log {

			t.Fatalf(
				"expected log %d at index %d, got %d",
				expected.log,
				index,
				actual.LogIndex,
			)
		}
	}

	if firstPage.NextCursor == nil {
		t.Fatal(
			"expected next cursor",
		)
	}

	secondPage, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 3,

				Cursor: firstPage.
					NextCursor,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected second page to succeed, got %v",
			err,
		)
	}

	if len(secondPage.Transfers) != 1 {
		t.Fatalf(
			"expected 1 transfer, got %d",
			len(secondPage.Transfers),
		)
	}

	if secondPage.Transfers[0].
		BlockNumber != 900000199 {

		t.Fatalf(
			"expected block 900000199, got %d",
			secondPage.Transfers[0].
				BlockNumber,
		)
	}

	if secondPage.Transfers[0].
		LogIndex != 50 {

		t.Fatalf(
			"expected log index 50, got %d",
			secondPage.Transfers[0].
				LogIndex,
		)
	}

	if secondPage.NextCursor != nil {
		t.Fatal(
			"expected no next cursor",
		)
	}
}

func TestPostgresTransferReaderCombinesCursorAndTokenFilter(
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

	const tokenA = "0xtest-token-a"

	const tokenB = "0xtest-token-b"

	_, err =
		pool.Exec(
			ctx,
			`
			DELETE FROM erc20_transfers
			WHERE token_address IN ($1, $2)
			`,
			tokenA,
			tokenB,
		)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	defer func() {
		_, _ =
			pool.Exec(
				context.Background(),
				`
				DELETE FROM erc20_transfers
				WHERE token_address IN ($1, $2)
				`,
				tokenA,
				tokenB,
			)
	}()

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	err =
		store.SaveBlock(
			ctx,
			domain.BlockTransferIndex{
				BlockNumber: 900000300,

				BlockHash: "0xtest-filter-block",

				Transfers: []domain.ERC20Transfer{
					{
						Token: tokenA,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(1),

						TransactionHash: "0xtest-filter-a1",

						LogIndex: 50,
					},
					{
						Token: tokenB,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(2),

						TransactionHash: "0xtest-filter-b1",

						LogIndex: 40,
					},
					{
						Token: tokenA,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(3),

						TransactionHash: "0xtest-filter-a2",

						LogIndex: 30,
					},
					{
						Token: tokenB,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(4),

						TransactionHash: "0xtest-filter-b2",

						LogIndex: 20,
					},
					{
						Token: tokenA,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(5),

						TransactionHash: "0xtest-filter-a3",

						LogIndex: 10,
					},
				},
			},
		)

	if err != nil {
		t.Fatalf(
			"failed to save transfers: %v",
			err,
		)
	}

	reader :=
		NewPostgresTransferReader(
			pool,
		)

	tokenAddress :=
		domain.Address(tokenA)

	firstPage, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 2,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected first page to succeed, got %v",
			err,
		)
	}

	if len(firstPage.Transfers) != 2 {
		t.Fatalf(
			"expected 2 token A transfers, got %d",
			len(firstPage.Transfers),
		)
	}

	for _, transfer := range firstPage.Transfers {

		if transfer.Token !=
			domain.Address(tokenA) {

			t.Fatalf(
				"expected only token A, got %s",
				transfer.Token,
			)
		}
	}

	if firstPage.Transfers[0].
		LogIndex != 50 {

		t.Fatalf(
			"expected log 50, got %d",
			firstPage.Transfers[0].
				LogIndex,
		)
	}

	if firstPage.Transfers[1].
		LogIndex != 30 {

		t.Fatalf(
			"expected log 30, got %d",
			firstPage.Transfers[1].
				LogIndex,
		)
	}

	if firstPage.NextCursor == nil {
		t.Fatal(
			"expected next cursor",
		)
	}

	secondPage, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 2,

				Cursor: firstPage.
					NextCursor,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected second page to succeed, got %v",
			err,
		)
	}

	if len(secondPage.Transfers) != 1 {
		t.Fatalf(
			"expected 1 remaining token A transfer, got %d",
			len(secondPage.Transfers),
		)
	}

	if secondPage.Transfers[0].
		Token != domain.Address(tokenA) {

		t.Fatalf(
			"expected token A, got %s",
			secondPage.Transfers[0].
				Token,
		)
	}

	if secondPage.Transfers[0].
		LogIndex != 10 {

		t.Fatalf(
			"expected log index 10, got %d",
			secondPage.Transfers[0].
				LogIndex,
		)
	}

	if secondPage.NextCursor != nil {
		t.Fatal(
			"expected no next cursor",
		)
	}
}

func TestPostgresTransferReaderHasNoCursorOnCompletePage(
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

	const token = "0xtest-final-page"

	_, err =
		pool.Exec(
			ctx,
			`
			DELETE FROM erc20_transfers
			WHERE token_address = $1
			`,
			token,
		)

	if err != nil {
		t.Fatalf(
			"failed to clean transfers: %v",
			err,
		)
	}

	defer func() {
		_, _ =
			pool.Exec(
				context.Background(),
				`
				DELETE FROM erc20_transfers
				WHERE token_address = $1
				`,
				token,
			)
	}()

	store :=
		NewPostgresBlockTransferStore(
			pool,
		)

	err =
		store.SaveBlock(
			ctx,
			domain.BlockTransferIndex{
				BlockNumber: 900000400,

				BlockHash: "0xtest-final-block",

				Transfers: []domain.ERC20Transfer{
					{
						Token: token,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(1),

						TransactionHash: "0xtest-final-1",

						LogIndex: 20,
					},
					{
						Token: token,

						From: "0xfrom",

						To: "0xto",

						Value: big.NewInt(2),

						TransactionHash: "0xtest-final-2",

						LogIndex: 10,
					},
				},
			},
		)

	if err != nil {
		t.Fatalf(
			"failed to save transfers: %v",
			err,
		)
	}

	reader :=
		NewPostgresTransferReader(
			pool,
		)

	tokenAddress :=
		domain.Address(token)

	page, err :=
		reader.ListTransfers(
			ctx,
			domain.TransferQuery{
				Token: &tokenAddress,

				Limit: 2,
			},
		)

	if err != nil {
		t.Fatalf(
			"expected query to succeed, got %v",
			err,
		)
	}

	if len(page.Transfers) != 2 {
		t.Fatalf(
			"expected 2 transfers, got %d",
			len(page.Transfers),
		)
	}

	if page.NextCursor != nil {
		t.Fatal(
			"expected no next cursor when all matching rows fit in one page",
		)
	}
}
