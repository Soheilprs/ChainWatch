package metadata

import (
	"context"
	"strings"
	"sync"

	"github.com/soheilprs/chainwatch/internal/domain"
)

type TokenMetadataService struct {
	fetcher TokenMetadataFetcher
	store   TokenMetadataStore

	mu    sync.RWMutex
	cache map[string]domain.TokenMetadata
}

func NewTokenMetadataService(
	fetcher TokenMetadataFetcher,
	store TokenMetadataStore,
) *TokenMetadataService {
	return &TokenMetadataService{
		fetcher: fetcher,
		store:   store,

		cache: make(
			map[string]domain.TokenMetadata,
		),
	}
}

func (s *TokenMetadataService) GetTokenMetadata(
	ctx context.Context,
	address domain.Address,
) (domain.TokenMetadata, error) {
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
		return domain.TokenMetadata{}, domain.NewDomainError(
			domain.ErrMetadata,
			"load cached token metadata",
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
		return domain.TokenMetadata{}, domain.NewDomainError(
			domain.ErrMetadata,
			"fetch token metadata",
			err,
		)
	}

	err =
		s.store.SaveTokenMetadata(
			ctx,
			metadata,
		)

	if err != nil {
		return domain.TokenMetadata{}, domain.NewDomainError(
			domain.ErrMetadata,
			"persist token metadata",
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
	metadata domain.TokenMetadata,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] =
		metadata
}
