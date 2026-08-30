package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresTransferReader struct {
	pool *pgxpool.Pool
}

var _ TransferReader = (*PostgresTransferReader)(nil)

func NewPostgresTransferReader(
	pool *pgxpool.Pool,
) *PostgresTransferReader {
	return &PostgresTransferReader{
		pool: pool,
	}
}

func (r *PostgresTransferReader) ListTransfers(
	ctx context.Context,
	query TransferQuery,
) ([]StoredERC20Transfer, error) {
	limit := query.Limit

	if limit <= 0 {
		limit = 100
	}

	if limit > 1000 {
		limit = 1000
	}

	sqlQuery := `
		SELECT
			block_number,
			block_hash,
			transaction_hash,
			log_index,
			token_address,
			from_address,
			to_address,
			value
		FROM erc20_transfers
	`

	conditions := make(
		[]string,
		0,
	)

	args := make(
		[]any,
		0,
	)

	addCondition := func(
		condition string,
		value any,
	) {
		args = append(
			args,
			value,
		)

		placeholder :=
			fmt.Sprintf(
				"$%d",
				len(args),
			)

		conditions = append(
			conditions,
			fmt.Sprintf(
				condition,
				placeholder,
			),
		)
	}

	if query.BlockNumber != nil {
		addCondition(
			"block_number = %s",
			int64(*query.BlockNumber),
		)
	}

	if query.Token != nil {
		addCondition(
			"LOWER(token_address) = LOWER(%s)",
			string(*query.Token),
		)
	}

	if query.Address != nil {
		args = append(
			args,
			string(*query.Address),
		)

		placeholder :=
			fmt.Sprintf(
				"$%d",
				len(args),
			)

		conditions = append(
			conditions,
			fmt.Sprintf(
				`(
					LOWER(from_address) = LOWER(%s)
					OR
					LOWER(to_address) = LOWER(%s)
				)`,
				placeholder,
				placeholder,
			),
		)
	}

	if len(conditions) > 0 {
		sqlQuery +=
			" WHERE " +
				strings.Join(
					conditions,
					" AND ",
				)
	}

	args = append(
		args,
		limit,
	)

	limitPlaceholder :=
		fmt.Sprintf(
			"$%d",
			len(args),
		)

	sqlQuery += fmt.Sprintf(
		`
		ORDER BY
			block_number DESC,
			log_index ASC
		LIMIT %s
		`,
		limitPlaceholder,
	)

	rows, err :=
		r.pool.Query(
			ctx,
			sqlQuery,
			args...,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to query transfers: %w",
			err,
		)
	}

	defer rows.Close()

	transfers := make(
		[]StoredERC20Transfer,
		0,
	)

	for rows.Next() {
		var blockNumber int64
		var blockHash string
		var transactionHash string
		var logIndex int64

		var token string
		var from string
		var to string

		var valueString string

		err := rows.Scan(
			&blockNumber,
			&blockHash,
			&transactionHash,
			&logIndex,
			&token,
			&from,
			&to,
			&valueString,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to scan transfer: %w",
				err,
			)
		}

		value, ok :=
			new(big.Int).SetString(
				valueString,
				10,
			)

		if !ok {
			return nil, fmt.Errorf(
				"invalid transfer value %q",
				valueString,
			)
		}

		transfers = append(
			transfers,
			StoredERC20Transfer{
				BlockNumber: uint64(blockNumber),

				BlockHash: BlockHash(blockHash),

				TransactionHash: TransactionHash(
					transactionHash,
				),

				LogIndex: uint(logIndex),

				Token: Address(token),

				From: Address(from),

				To: Address(to),

				Value: value,
			},
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"failed while reading transfers: %w",
			err,
		)
	}

	return transfers, nil
}
