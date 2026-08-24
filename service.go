package main

import (
	"context"
	"fmt"
)

type ChainWatchService struct {
	client     BlockchainClient
	blockchain *Blockchain
}

func NewChainWatchService(
	client BlockchainClient,
	blockchain *Blockchain,
) *ChainWatchService {
	return &ChainWatchService{
		client:     client,
		blockchain: blockchain,
	}
}

func (s *ChainWatchService) SyncLatestBlock(
	ctx context.Context,
) error {
	block, err := s.client.GetLatestBlock(ctx)

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
