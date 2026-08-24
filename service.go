package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ChainWatchService struct {
	client      BlockchainClient
	blockchain  *Blockchain
	maxAttempts int
	retryDelay  time.Duration
}

func NewChainWatchService(
	client BlockchainClient,
	blockchain *Blockchain,
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
) (Block, error) {
	var lastErr error

	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		block, err := s.client.GetLatestBlock(ctx)

		if err == nil {
			return block, nil
		}

		lastErr = err

		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return Block{}, err
		}

		if attempt == s.maxAttempts {
			break
		}

		delay := s.retryDelay * time.Duration(attempt)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return Block{}, ctx.Err()
		}
	}

	return Block{}, fmt.Errorf(
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
