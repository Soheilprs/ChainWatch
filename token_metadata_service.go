package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type TokenMetadataService struct {
	fetcher TokenMetadataFetcher
	store   TokenMetadataStore

	mu    sync.RWMutex
	cache map[string]TokenMetadata
}

var _ TokenMetadataProvider = (*TokenMetadataService)(nil)

func NewTokenMetadataService(
	fetcher TokenMetadataFetcher,
	store TokenMetadataStore,
) *TokenMetadataService {
	return &TokenMetadataService{
		fetcher: fetcher,
		store:   store,

		cache: make(
			map[string]TokenMetadata,
		),
	}
}

func (s *TokenMetadataService) GetTokenMetadata(
	ctx context.Context,
	address Address,
) (TokenMetadata, error) {
	key :=
		strings.ToLower(
			string(address),
		)

	s.mu.RLock()

	metadata, exists :=
		s.cache[key]

	s.mu.RUnlock()

	if exists {
		return metadata, nil
	}

	metadata, exists, err :=
		s.store.LoadTokenMetadata(
			ctx,
			address,
		)

	if err != nil {
		return TokenMetadata{},
			fmt.Errorf(
				"failed to load cached token metadata: %w",
				err,
			)
	}

	if exists {
		s.cacheMetadata(
			key,
			metadata,
		)

		return metadata, nil
	}

	metadata, err =
		s.fetcher.FetchTokenMetadata(
			ctx,
			address,
		)

	if err != nil {
		return TokenMetadata{},
			fmt.Errorf(
				"failed to fetch token metadata: %w",
				err,
			)
	}

	err =
		s.store.SaveTokenMetadata(
			ctx,
			metadata,
		)

	if err != nil {
		return TokenMetadata{},
			fmt.Errorf(
				"failed to persist token metadata: %w",
				err,
			)
	}

	s.cacheMetadata(
		key,
		metadata,
	)

	return metadata, nil
}

func (s *TokenMetadataService) cacheMetadata(
	key string,
	metadata TokenMetadata,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] =
		metadata
}
