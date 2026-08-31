package service

import (
	"context"
	"errors"
	"time"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type MockBlockchainClient struct {
	Block        domain.Block
	Err          error
	Delay        time.Duration
	FailuresLeft int
	Calls        int
}

var _ BlockchainClient = (*MockBlockchainClient)(nil)

func (m *MockBlockchainClient) GetLatestBlock(ctx context.Context) (domain.Block, error) {
	m.Calls++
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return domain.Block{}, ctx.Err()
		}
	}
	if m.FailuresLeft > 0 {
		m.FailuresLeft--
		if m.Err != nil {
			return domain.Block{}, m.Err
		}
		return domain.Block{}, errors.New("temporary rpc failure")
	}
	return m.Block, nil
}
