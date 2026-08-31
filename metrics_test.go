package main

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsRecordsHTTPRequest(
	t *testing.T,
) {
	metrics :=
		NewMetrics()

	metrics.ObserveHTTPRequest(
		200,
		150*time.Millisecond,
	)

	snapshot :=
		metrics.Snapshot()

	if snapshot.HTTPRequests != 1 {
		t.Fatalf(
			"expected 1 HTTP request, got %d",
			snapshot.HTTPRequests,
		)
	}

	if snapshot.HTTPErrors != 0 {
		t.Fatalf(
			"expected 0 HTTP errors, got %d",
			snapshot.HTTPErrors,
		)
	}

	if snapshot.HTTPDurationNanos == 0 {
		t.Fatal(
			"expected HTTP duration to be recorded",
		)
	}
}

func TestMetricsRecordsHTTPServerError(
	t *testing.T,
) {
	metrics :=
		NewMetrics()

	metrics.ObserveHTTPRequest(
		500,
		time.Millisecond,
	)

	snapshot :=
		metrics.Snapshot()

	if snapshot.HTTPRequests != 1 {
		t.Fatalf(
			"expected 1 request, got %d",
			snapshot.HTTPRequests,
		)
	}

	if snapshot.HTTPErrors != 1 {
		t.Fatalf(
			"expected 1 HTTP error, got %d",
			snapshot.HTTPErrors,
		)
	}
}

func TestMetricsRecordsIndexedBlock(
	t *testing.T,
) {
	metrics :=
		NewMetrics()

	metrics.RecordIndexedBlock(
		123,
	)

	snapshot :=
		metrics.Snapshot()

	if snapshot.IndexedBlocks != 1 {
		t.Fatalf(
			"expected 1 indexed block, got %d",
			snapshot.IndexedBlocks,
		)
	}

	if snapshot.IndexedTransfers != 123 {
		t.Fatalf(
			"expected 123 indexed transfers, got %d",
			snapshot.IndexedTransfers,
		)
	}
}

func TestMetricsPrometheusOutput(
	t *testing.T,
) {
	metrics :=
		NewMetrics()

	metrics.RecordIndexedBlock(
		42,
	)

	metrics.RecordIndexerError()

	var output strings.Builder

	err :=
		metrics.WritePrometheus(
			&output,
		)

	if err != nil {
		t.Fatalf(
			"failed to render metrics: %v",
			err,
		)
	}

	result :=
		output.String()

	expected :=
		[]string{
			"chainwatch_indexed_blocks_total 1",
			"chainwatch_indexed_transfers_total 42",
			"chainwatch_indexer_errors_total 1",
		}

	for _, value := range expected {

		if !strings.Contains(
			result,
			value,
		) {
			t.Fatalf(
				"expected output to contain %q",
				value,
			)
		}
	}
}
