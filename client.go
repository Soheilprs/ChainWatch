package main

import (
	"context"
	"errors"
	"time"
)

type BlockchainClient interface {
	GetLatestBlock(ctx context.Context) (Block, error)
}

type MockBlockchainClient struct {
	Block        Block
	Err          error
	Delay        time.Duration
	FailuresLeft int
	Calls        int
}

var _ BlockchainClient = (*MockBlockchainClient)(nil)

func (m *MockBlockchainClient) GetLatestBlock(
	ctx context.Context,
) (Block, error) {
	m.Calls++

	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return Block{}, ctx.Err()
		}
	}

	if m.FailuresLeft > 0 {
		m.FailuresLeft--

		if m.Err != nil {
			return Block{}, m.Err
		}

		return Block{}, errors.New(
			"temporary rpc failure",
		)
	}

	return m.Block, nil
}
