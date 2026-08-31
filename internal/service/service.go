package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type ChainWatchService struct {
	client      BlockchainClient
	blockchain  *domain.Blockchain
	maxAttempts int
	retryDelay  time.Duration
}

func NewChainWatchService(
	client BlockchainClient,
	blockchain *domain.Blockchain,
) *ChainWatchService {
	return &ChainWatchService{
		client:      client,
		blockchain:  blockchain,
		maxAttempts: 3,
		retryDelay:  50 * time.Millisecond,
	}
}

func (s *ChainWatchService) fetchLatestBlockWithRetry(
	ctx context.Context,
) (domain.Block, error) {
	var lastErr error

	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		block, err := s.client.GetLatestBlock(ctx)

		if err == nil {
			return block, nil
		}

		lastErr = err

		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return domain.Block{}, err
		}

		if attempt == s.maxAttempts {
			break
		}

		delay := s.retryDelay * time.Duration(attempt)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return domain.Block{}, ctx.Err()
		}
	}

	return domain.Block{}, fmt.Errorf(
		"failed after %d attempts: %w",
		s.maxAttempts,
		lastErr,
	)
}

func (s *ChainWatchService) SyncLatestBlock(
	ctx context.Context,
) error {
	block, err := s.fetchLatestBlockWithRetry(ctx)

	if err != nil {
		return fmt.Errorf(
			"failed to fetch latest block: %w",
			err,
		)
	}

	if err := s.blockchain.ProcessBlock(block); err != nil {
		return fmt.Errorf(
			"failed to process latest block: %w",
			err,
		)
	}

	return nil
}
