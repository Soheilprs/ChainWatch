package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
)

type RPCResilienceConfig struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	JitterFraction    float64
	RequestsPerSecond int
	Burst             int
}

func (c RPCResilienceConfig) Validate() error {
	switch {
	case c.MaxAttempts <= 0:
		return errors.New("RPC max attempts must be greater than zero")
	case c.InitialBackoff <= 0:
		return errors.New("RPC initial backoff must be greater than zero")
	case c.MaxBackoff < c.InitialBackoff:
		return errors.New("RPC max backoff must be at least the initial backoff")
	case c.JitterFraction < 0 || c.JitterFraction > 1:
		return errors.New("RPC jitter fraction must be between zero and one")
	case c.RequestsPerSecond <= 0:
		return errors.New("RPC requests per second must be greater than zero")
	case c.Burst <= 0:
		return errors.New("RPC burst must be greater than zero")
	default:
		return nil
	}
}

type resilientEthereumBackend interface {
	ContinuousTransferClient
	TokenMetadataFetcher
}

type ResilientEthereumClient struct {
	next    resilientEthereumBackend
	policy  rpcRetryPolicy
	limiter *rpcRateLimiter
	metrics *Metrics
}

var _ ContinuousTransferClient = (*ResilientEthereumClient)(nil)
var _ TokenMetadataFetcher = (*ResilientEthereumClient)(nil)

func NewResilientEthereumClient(
	next resilientEthereumBackend,
	config RPCResilienceConfig,
	metrics *Metrics,
) (*ResilientEthereumClient, error) {
	if next == nil {
		return nil, errors.New("Ethereum RPC client is required")
	}
	if err := config.Validate(); err != nil {
		return nil, NewDomainError(ErrValidation, "validate RPC resilience configuration", err)
	}
	if metrics == nil {
		metrics = NewMetrics()
	}

	return &ResilientEthereumClient{
		next: next,
		policy: rpcRetryPolicy{
			maxAttempts:    config.MaxAttempts,
			initialBackoff: config.InitialBackoff,
			maxBackoff:     config.MaxBackoff,
			jitterFraction: config.JitterFraction,
			random:         rand.Float64,
			sleep:          sleepWithContext,
		},
		limiter: newRPCRateLimiter(config.RequestsPerSecond, config.Burst),
		metrics: metrics,
	}, nil
}

func (c *ResilientEthereumClient) GetLatestObservedBlock(
	ctx context.Context,
) (ObservedBlock, error) {
	return executeResilientRPC(ctx, c, func(ctx context.Context) (ObservedBlock, error) {
		return c.next.GetLatestObservedBlock(ctx)
	})
}

func (c *ResilientEthereumClient) GetObservedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (ObservedBlock, error) {
	return executeResilientRPC(ctx, c, func(ctx context.Context) (ObservedBlock, error) {
		return c.next.GetObservedBlockByNumber(ctx, blockNumber)
	})
}

func (c *ResilientEthereumClient) GetERC20TransfersByBlock(
	ctx context.Context,
	block ObservedBlock,
) (BlockTransferIndex, error) {
	return executeResilientRPC(ctx, c, func(ctx context.Context) (BlockTransferIndex, error) {
		return c.next.GetERC20TransfersByBlock(ctx, block)
	})
}

func (c *ResilientEthereumClient) FetchTokenMetadata(
	ctx context.Context,
	address Address,
) (TokenMetadata, error) {
	return executeResilientRPC(ctx, c, func(ctx context.Context) (TokenMetadata, error) {
		return c.next.FetchTokenMetadata(ctx, address)
	})
}

func executeResilientRPC[T any](
	ctx context.Context,
	client *ResilientEthereumClient,
	call func(context.Context) (T, error),
) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= client.policy.maxAttempts; attempt++ {
		if err := client.limiter.Wait(ctx); err != nil {
			return zero, err
		}

		client.metrics.RecordRPCRequest()
		value, err := call(ctx)
		if err == nil {
			return value, nil
		}
		lastErr = err

		if ctxErr := ctx.Err(); ctxErr != nil {
			return zero, ctxErr
		}
		if !isRetryableRPCError(err) {
			client.metrics.RecordRPCFailure()
			return zero, err
		}
		if attempt == client.policy.maxAttempts {
			client.metrics.RecordRPCFailure()
			return zero, NewDomainError(
				ErrTemporaryDependency,
				fmt.Sprintf("RPC retry budget exhausted after %d attempts", attempt),
				lastErr,
			)
		}

		client.metrics.RecordRPCRetry()
		if err := client.policy.wait(ctx, attempt); err != nil {
			return zero, err
		}
	}

	return zero, lastErr
}

type rpcRetryPolicy struct {
	maxAttempts    int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	jitterFraction float64
	random         func() float64
	sleep          func(context.Context, time.Duration) error
}

func (p rpcRetryPolicy) wait(ctx context.Context, failedAttempt int) error {
	delay := p.initialBackoff
	for exponent := 1; exponent < failedAttempt && delay < p.maxBackoff; exponent++ {
		if delay > p.maxBackoff/2 {
			delay = p.maxBackoff
			break
		}
		delay *= 2
	}
	if delay > p.maxBackoff {
		delay = p.maxBackoff
	}

	if p.jitterFraction > 0 {
		random := 0.5
		if p.random != nil {
			random = p.random()
		}
		multiplier := 1 + p.jitterFraction*(2*random-1)
		delay = time.Duration(math.Max(0, float64(delay)*multiplier))
	}
	return p.sleep(ctx, delay)
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type rpcRateLimiter struct {
	mu sync.Mutex

	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	now    func() time.Time
}

func newRPCRateLimiter(requestsPerSecond int, burst int) *rpcRateLimiter {
	now := time.Now()
	return &rpcRateLimiter{
		rate:   float64(requestsPerSecond),
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now,
		now:    time.Now,
	}
}

func (l *rpcRateLimiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := l.now()
		elapsed := now.Sub(l.last).Seconds()
		if elapsed > 0 {
			l.tokens = math.Min(l.burst, l.tokens+elapsed*l.rate)
			l.last = now
		}
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - l.tokens) / l.rate * float64(time.Second))
		l.mu.Unlock()

		if err := sleepWithContext(ctx, wait); err != nil {
			return err
		}
	}
}

func isRetryableRPCError(err error) bool {
	if err == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, ErrValidation) ||
		errors.Is(err, ErrInvalidTokenAddress) ||
		errors.Is(err, ErrChainReorg) {
		return false
	}
	if errors.Is(err, ErrTemporaryDependency) ||
		errors.Is(err, ethereum.NotFound) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	var httpError rpc.HTTPError
	if errors.As(err, &httpError) {
		return retryableHTTPStatus(httpError.StatusCode)
	}
	var httpErrorPointer *rpc.HTTPError
	if errors.As(err, &httpErrorPointer) {
		return retryableHTTPStatus(httpErrorPointer.StatusCode)
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return networkError.Timeout() || networkError.Temporary()
	}
	var rpcError rpc.Error
	if errors.As(err, &rpcError) {
		return rpcError.ErrorCode() == -32002 || rpcError.ErrorCode() == -32603
	}
	return false
}

func retryableHTTPStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusTooEarly ||
		status == http.StatusTooManyRequests ||
		status >= http.StatusInternalServerError
}
