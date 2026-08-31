package store

import (
	"context"
	"fmt"
	"github.com/soheilprs/chainwatch/internal/domain"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresTransferReader struct {
	pool *pgxpool.Pool
}

func NewPostgresTransferReader(
	pool *pgxpool.Pool,
) *PostgresTransferReader {
	return &PostgresTransferReader{
		pool: pool,
	}
}

func (r *PostgresTransferReader) ListTransfers(
	ctx context.Context,
	query domain.TransferQuery,
) (page domain.TransferPage, err error) {
	defer domain.ClassifyError(&err, domain.ErrDatabase, "list transfers")

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

	addArgument := func(
		value any,
	) string {
		args = append(
			args,
			value,
		)

		return fmt.Sprintf(
			"$%d",
			len(args),
		)
	}

	if query.BlockNumber != nil {
		placeholder :=
			addArgument(
				int64(
					*query.BlockNumber,
				),
			)

		conditions = append(
			conditions,
			fmt.Sprintf(
				"block_number = %s",
				placeholder,
			),
		)
	}

	if query.Token != nil {
		placeholder :=
			addArgument(
				string(
					*query.Token,
				),
			)

		conditions = append(
			conditions,
			fmt.Sprintf(
				"LOWER(token_address) = LOWER(%s)",
				placeholder,
			),
		)
	}

	if query.Address != nil {
		placeholder :=
			addArgument(
				string(
					*query.Address,
				),
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

	if query.Cursor != nil {
		blockPlaceholder :=
			addArgument(
				int64(
					query.Cursor.
						BlockNumber,
				),
			)

		logPlaceholder :=
			addArgument(
				int64(
					query.Cursor.
						LogIndex,
				),
			)

		conditions = append(
			conditions,
			fmt.Sprintf(
				`(
					block_number < %s
					OR (
						block_number = %s
						AND log_index < %s
					)
				)`,
				blockPlaceholder,
				blockPlaceholder,
				logPlaceholder,
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

	queryLimit :=
		limit + 1

	limitPlaceholder :=
		addArgument(
			queryLimit,
		)

	sqlQuery += fmt.Sprintf(
		`
		ORDER BY
			block_number DESC,
			log_index DESC
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
		return domain.TransferPage{},
			fmt.Errorf(
				"failed to query transfers: %w",
				err,
			)
	}

	defer rows.Close()

	transfers := make(
		[]domain.StoredERC20Transfer,
		0,
		queryLimit,
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
			return domain.TransferPage{},
				fmt.Errorf(
					"failed to scan transfer: %w",
					err,
				)
		}

		if blockNumber < 0 {
			return domain.TransferPage{},
				fmt.Errorf(
					"invalid negative block number: %d",
					blockNumber,
				)
		}

		if logIndex < 0 {
			return domain.TransferPage{},
				fmt.Errorf(
					"invalid negative log index: %d",
					logIndex,
				)
		}

		value, ok :=
			new(big.Int).SetString(
				valueString,
				10,
			)

		if !ok {
			return domain.TransferPage{},
				fmt.Errorf(
					"invalid transfer value %q",
					valueString,
				)
		}

		transfers = append(
			transfers,
			domain.StoredERC20Transfer{
				BlockNumber: uint64(blockNumber),

				BlockHash: domain.BlockHash(
					blockHash,
				),

				TransactionHash: domain.TransactionHash(
					transactionHash,
				),

				LogIndex: uint(logIndex),

				Token: domain.Address(token),

				From: domain.Address(from),

				To: domain.Address(to),

				Value: value,
			},
		)
	}

	if err := rows.Err(); err != nil {
		return domain.TransferPage{},
			fmt.Errorf(
				"failed while reading transfers: %w",
				err,
			)
	}

	page = domain.TransferPage{
		Transfers: transfers,
	}

	if len(transfers) > limit {
		page.Transfers =
			transfers[:limit]

		lastTransfer :=
			page.Transfers[len(page.Transfers)-1]

		page.NextCursor =
			&domain.TransferCursor{
				BlockNumber: lastTransfer.
					BlockNumber,

				LogIndex: lastTransfer.
					LogIndex,
			}
	}

	return page, nil
}
