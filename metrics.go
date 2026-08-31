package main

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type Metrics struct {
	httpRequests      atomic.Uint64
	httpErrors        atomic.Uint64
	httpDurationNanos atomic.Uint64

	indexedBlocks    atomic.Uint64
	indexedTransfers atomic.Uint64
	indexerErrors    atomic.Uint64

	tokenMetadataErrors atomic.Uint64
}

type MetricsSnapshot struct {
	HTTPRequests      uint64
	HTTPErrors        uint64
	HTTPDurationNanos uint64

	IndexedBlocks    uint64
	IndexedTransfers uint64
	IndexerErrors    uint64

	TokenMetadataErrors uint64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) ObserveHTTPRequest(
	status int,
	duration time.Duration,
) {
	m.httpRequests.Add(1)

	m.httpDurationNanos.Add(
		uint64(duration),
	)

	if status >= 500 {
		m.httpErrors.Add(1)
	}
}

func (m *Metrics) RecordIndexedBlock(
	transferCount int,
) {
	m.indexedBlocks.Add(1)

	if transferCount > 0 {
		m.indexedTransfers.Add(
			uint64(transferCount),
		)
	}
}

func (m *Metrics) RecordIndexerError() {
	m.indexerErrors.Add(1)
}

func (m *Metrics) RecordTokenMetadataError() {
	m.tokenMetadataErrors.Add(1)
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		HTTPRequests: m.httpRequests.Load(),

		HTTPErrors: m.httpErrors.Load(),

		HTTPDurationNanos: m.httpDurationNanos.Load(),

		IndexedBlocks: m.indexedBlocks.Load(),

		IndexedTransfers: m.indexedTransfers.Load(),

		IndexerErrors: m.indexerErrors.Load(),

		TokenMetadataErrors: m.tokenMetadataErrors.Load(),
	}
}

func (m *Metrics) WritePrometheus(
	w io.Writer,
) error {
	snapshot :=
		m.Snapshot()

	durationSeconds :=
		float64(
			snapshot.HTTPDurationNanos,
		) /
			float64(time.Second)

	_, err :=
		fmt.Fprintf(
			w,
			`# HELP chainwatch_http_requests_total Total HTTP requests handled.
# TYPE chainwatch_http_requests_total counter
chainwatch_http_requests_total %d

# HELP chainwatch_http_errors_total Total HTTP requests returning 5xx.
# TYPE chainwatch_http_errors_total counter
chainwatch_http_errors_total %d

# HELP chainwatch_http_request_duration_seconds_sum Total HTTP request duration in seconds.
# TYPE chainwatch_http_request_duration_seconds_sum counter
chainwatch_http_request_duration_seconds_sum %f

# HELP chainwatch_http_request_duration_seconds_count Total HTTP request duration observations.
# TYPE chainwatch_http_request_duration_seconds_count counter
chainwatch_http_request_duration_seconds_count %d

# HELP chainwatch_indexed_blocks_total Total successfully indexed blocks.
# TYPE chainwatch_indexed_blocks_total counter
chainwatch_indexed_blocks_total %d

# HELP chainwatch_indexed_transfers_total Total indexed ERC-20 transfers.
# TYPE chainwatch_indexed_transfers_total counter
chainwatch_indexed_transfers_total %d

# HELP chainwatch_indexer_errors_total Total indexer errors.
# TYPE chainwatch_indexer_errors_total counter
chainwatch_indexer_errors_total %d

# HELP chainwatch_token_metadata_errors_total Total token metadata lookup errors.
# TYPE chainwatch_token_metadata_errors_total counter
chainwatch_token_metadata_errors_total %d
`,
			snapshot.HTTPRequests,
			snapshot.HTTPErrors,
			durationSeconds,
			snapshot.HTTPRequests,
			snapshot.IndexedBlocks,
			snapshot.IndexedTransfers,
			snapshot.IndexerErrors,
			snapshot.TokenMetadataErrors,
		)

	return err
}
