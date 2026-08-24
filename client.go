package main

import (
	"context"
	"time"
)

type BlockchainClient interface {
	GetLatestBlock(ctx context.Context) (Block, error)
}

type MockBlockchainClient struct {
	Block Block
	Err   error
	Delay time.Duration
}

var _ BlockchainClient = MockBlockchainClient{}

func (m MockBlockchainClient) GetLatestBlock(
	ctx context.Context,
) (Block, error) {
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return Block{}, ctx.Err()
		}
	}

	if m.Err != nil {
		return Block{}, m.Err
	}

	return m.Block, nil
}
