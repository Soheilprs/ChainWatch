package main

import "context"

type RangeIndexer interface {
	IndexRange(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
	) ([]BlockTransferIndex, error)
}
