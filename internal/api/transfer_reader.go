package api

import (
	"context"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type TransferReader interface {
	ListTransfers(
		ctx context.Context,
		query domain.TransferQuery,
	) (domain.TransferPage, error)
}

// TokenMetadataProvider enriches transfer responses without coupling the API
// to a concrete cache or Ethereum client.
type TokenMetadataProvider interface {
	GetTokenMetadata(context.Context, domain.Address) (domain.TokenMetadata, error)
}
