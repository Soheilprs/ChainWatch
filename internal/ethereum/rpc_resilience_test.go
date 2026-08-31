package ethereum

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/soheilprs/chainwatch/internal/domain"
	"github.com/soheilprs/chainwatch/internal/observability"
)

type retryableTestError struct{}

func (retryableTestError) Error() string   { return "temporary network error" }
func (retryableTestError) Timeout() bool   { return false }
func (retryableTestError) Temporary() bool { return true }

func TestExecuteResilientRPCRetriesTemporaryFailures(t *testing.T) {
	metrics := observability.NewMetrics()
	var delays []time.Duration
	client := newTestResilientClient(metrics, 3)
	client.policy.initialBackoff = 10 * time.Millisecond
	client.policy.maxBackoff = 15 * time.Millisecond
	client.policy.sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	calls := 0
	value, err := executeResilientRPC(
		context.Background(),
		client,
		func(context.Context) (int, error) {
			calls++
			if calls < 3 {
				return 0, retryableTestError{}
			}
			return 42, nil
		},
	)
	if err != nil || value != 42 {
		t.Fatalf("value=%d err=%v", value, err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	if len(delays) != 2 || delays[0] != 10*time.Millisecond || delays[1] != 15*time.Millisecond {
		t.Fatalf("unexpected backoff delays: %v", delays)
	}
	snapshot := metrics.Snapshot()
	if snapshot.RPCRequests != 3 || snapshot.RPCRetries != 2 || snapshot.RPCFailures != 0 {
		t.Fatalf("unexpected RPC metrics: %+v", snapshot)
	}
}

func TestExecuteResilientRPCDoesNotRetryPermanentFailure(t *testing.T) {
	metrics := observability.NewMetrics()
	client := newTestResilientClient(metrics, 4)
	calls := 0

	_, err := executeResilientRPC(
		context.Background(),
		client,
		func(context.Context) (int, error) {
			calls++
			return 0, domain.ErrInvalidTokenAddress
		},
	)
	if !errors.Is(err, domain.ErrInvalidTokenAddress) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	snapshot := metrics.Snapshot()
	if snapshot.RPCRequests != 1 || snapshot.RPCRetries != 0 || snapshot.RPCFailures != 1 {
		t.Fatalf("unexpected RPC metrics: %+v", snapshot)
	}
}

func TestExecuteResilientRPCExhaustsBoundedRetryBudget(t *testing.T) {
	metrics := observability.NewMetrics()
	client := newTestResilientClient(metrics, 2)
	cause := retryableTestError{}

	_, err := executeResilientRPC(
		context.Background(),
		client,
		func(context.Context) (int, error) {
			return 0, cause
		},
	)
	if !errors.Is(err, domain.ErrTemporaryDependency) || !errors.Is(err, cause) {
		t.Fatalf("expected exhausted temporary failure, got %v", err)
	}
	snapshot := metrics.Snapshot()
	if snapshot.RPCRequests != 2 || snapshot.RPCRetries != 1 || snapshot.RPCFailures != 1 {
		t.Fatalf("unexpected RPC metrics: %+v", snapshot)
	}
}

func TestRPCRateLimiterHonorsContextCancellation(t *testing.T) {
	limiter := newRPCRateLimiter(1, 1)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("consume initial token: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestRetryableRPCErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{name: "rate limited", err: rpc.HTTPError{StatusCode: http.StatusTooManyRequests}, retryable: true},
		{name: "server error", err: rpc.HTTPError{StatusCode: http.StatusBadGateway}, retryable: true},
		{name: "bad request", err: rpc.HTTPError{StatusCode: http.StatusBadRequest}, retryable: false},
		{name: "deadline", err: context.DeadlineExceeded, retryable: false},
		{name: "validation", err: domain.NewDomainError(domain.ErrValidation, "validate", errors.New("bad input")), retryable: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := isRetryableRPCError(test.err); actual != test.retryable {
				t.Fatalf("retryable=%v, want %v", actual, test.retryable)
			}
		})
	}
}

func newTestResilientClient(metrics *observability.Metrics, maxAttempts int) *ResilientEthereumClient {
	return &ResilientEthereumClient{
		policy: rpcRetryPolicy{
			maxAttempts:    maxAttempts,
			initialBackoff: time.Millisecond,
			maxBackoff:     time.Millisecond,
			sleep: func(context.Context, time.Duration) error {
				return nil
			},
		},
		limiter: newRPCRateLimiter(1000, 1000),
		metrics: metrics,
	}
}
