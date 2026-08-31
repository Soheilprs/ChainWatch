package service

import (
	"context"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type BlockchainClient interface {
	GetLatestBlock(ctx context.Context) (domain.Block, error)
}
