package main

import (
	"context"
	"errors"
	"testing"
)

type MockTokenMetadataFetcher struct {
	Metadata TokenMetadata
	Err      error
	Calls    int
}

func (m *MockTokenMetadataFetcher) FetchTokenMetadata(
	ctx context.Context,
	address Address,
) (TokenMetadata, error) {
	m.Calls++

	if m.Err != nil {
		return TokenMetadata{},
			m.Err
	}

	return m.Metadata, nil
}

type MockTokenMetadataStore struct {
	Metadata TokenMetadata
	Exists   bool
	Err      error

	LoadCalls int
	SaveCalls int
}

func (m *MockTokenMetadataStore) LoadTokenMetadata(
	ctx context.Context,
	address Address,
) (TokenMetadata, bool, error) {
	m.LoadCalls++
	if m.Err != nil {
		return TokenMetadata{}, false, m.Err
	}

	return m.Metadata,
		m.Exists,
		nil
}

func (m *MockTokenMetadataStore) SaveTokenMetadata(
	ctx context.Context,
	metadata TokenMetadata,
) error {
	m.SaveCalls++

	m.Metadata =
		metadata

	m.Exists =
		true

	return nil
}

func TestTokenMetadataServiceFetchesAndCachesMetadata(
	t *testing.T,
) {
	address :=
		Address(
			"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		)

	fetcher :=
		&MockTokenMetadataFetcher{
			Metadata: TokenMetadata{
				Address: address,

				Name: "USD Coin",

				Symbol: "USDC",

				Decimals: 6,
			},
		}

	store :=
		&MockTokenMetadataStore{}

	service :=
		NewTokenMetadataService(
			fetcher,
			store,
		)

	ctx :=
		context.Background()

	first, err :=
		service.GetTokenMetadata(
			ctx,
			address,
		)

	if err != nil {
		t.Fatalf(
			"first metadata request failed: %v",
			err,
		)
	}

	second, err :=
		service.GetTokenMetadata(
			ctx,
			address,
		)

	if err != nil {
		t.Fatalf(
			"second metadata request failed: %v",
			err,
		)
	}

	if first.Symbol != "USDC" ||
		second.Symbol != "USDC" {

		t.Fatal(
			"expected USDC metadata",
		)
	}

	if fetcher.Calls != 1 {
		t.Fatalf(
			"expected RPC fetcher to be called once, got %d",
			fetcher.Calls,
		)
	}

	if store.SaveCalls != 1 {
		t.Fatalf(
			"expected metadata to be persisted once, got %d",
			store.SaveCalls,
		)
	}

	if store.LoadCalls != 1 {
		t.Fatalf(
			"expected database load once, got %d",
			store.LoadCalls,
		)
	}
}

func TestTokenMetadataServiceUsesDatabaseBeforeRPC(
	t *testing.T,
) {
	address :=
		Address(
			"0xdAC17F958D2ee523a2206206994597C13D831ec7",
		)

	fetcher :=
		&MockTokenMetadataFetcher{}

	store :=
		&MockTokenMetadataStore{
			Exists: true,

			Metadata: TokenMetadata{
				Address: address,

				Name: "Tether USD",

				Symbol: "USDT",

				Decimals: 6,
			},
		}

	service :=
		NewTokenMetadataService(
			fetcher,
			store,
		)

	metadata, err :=
		service.GetTokenMetadata(
			context.Background(),
			address,
		)

	if err != nil {
		t.Fatalf(
			"metadata request failed: %v",
			err,
		)
	}

	if metadata.Symbol != "USDT" {
		t.Fatalf(
			"expected USDT, got %s",
			metadata.Symbol,
		)
	}

	if fetcher.Calls != 0 {
		t.Fatalf(
			"expected no RPC call, got %d",
			fetcher.Calls,
		)
	}

	if store.LoadCalls != 1 {
		t.Fatalf(
			"expected one database load, got %d",
			store.LoadCalls,
		)
	}
}

func TestTokenMetadataServiceCacheIsCaseInsensitive(
	t *testing.T,
) {
	lower :=
		Address(
			"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		)

	checksummed :=
		Address(
			"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		)

	fetcher :=
		&MockTokenMetadataFetcher{
			Metadata: TokenMetadata{
				Address: checksummed,

				Name: "USD Coin",

				Symbol: "USDC",

				Decimals: 6,
			},
		}

	store :=
		&MockTokenMetadataStore{}

	service :=
		NewTokenMetadataService(
			fetcher,
			store,
		)

	_, err :=
		service.GetTokenMetadata(
			context.Background(),
			lower,
		)

	if err != nil {
		t.Fatalf(
			"first request failed: %v",
			err,
		)
	}

	_, err =
		service.GetTokenMetadata(
			context.Background(),
			checksummed,
		)

	if err != nil {
		t.Fatalf(
			"second request failed: %v",
			err,
		)
	}

	if fetcher.Calls != 1 {
		t.Fatalf(
			"expected only one RPC fetch, got %d",
			fetcher.Calls,
		)
	}
}

func TestTokenMetadataServiceClassifiesFailures(t *testing.T) {
	cause := errors.New("metadata database unavailable")
	service := NewTokenMetadataService(
		&MockTokenMetadataFetcher{},
		&MockTokenMetadataStore{Err: cause},
	)

	_, err := service.GetTokenMetadata(
		context.Background(),
		"0x1111111111111111111111111111111111111111",
	)
	if !errors.Is(err, ErrMetadata) {
		t.Fatalf("expected metadata classification, got %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected original cause, got %v", err)
	}
}
