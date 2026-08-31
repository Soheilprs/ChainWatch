package main

import "context"

type TransferReader interface {
	ListTransfers(
		ctx context.Context,
		query TransferQuery,
	) (TransferPage, error)
}
