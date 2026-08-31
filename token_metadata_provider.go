package main

import "context"

type TokenMetadataFetcher interface {
	FetchTokenMetadata(
		ctx context.Context,
		address Address,
	) (TokenMetadata, error)
}

type TokenMetadataStore interface {
	LoadTokenMetadata(
		ctx context.Context,
		address Address,
	) (TokenMetadata, bool, error)

	SaveTokenMetadata(
		ctx context.Context,
		metadata TokenMetadata,
	) error
}

type TokenMetadataProvider interface {
	GetTokenMetadata(
		ctx context.Context,
		address Address,
	) (TokenMetadata, error)
}
