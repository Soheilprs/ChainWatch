package metadata

import (
	"context"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type TokenMetadataFetcher interface {
	FetchTokenMetadata(
		ctx context.Context,
		address domain.Address,
	) (domain.TokenMetadata, error)
}

type TokenMetadataStore interface {
	LoadTokenMetadata(
		ctx context.Context,
		address domain.Address,
	) (domain.TokenMetadata, bool, error)

	SaveTokenMetadata(
		ctx context.Context,
		metadata domain.TokenMetadata,
	) error
}
