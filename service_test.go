package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChainWatchServiceSyncLatestBlock(t *testing.T) {
	blockchain := newTestBlockchain()

	block := Block{
		Number: 1,
		Hash:   "0xblock-service",
		Transactions: []Transaction{
			{
				Hash:     "0xtx-service",
				From:     "0xAlice",
				To:       "0xBob",
				ValueWei: 10,
			},
		},
	}

	client := MockBlockchainClient{
		Block: block,
	}

	service := NewChainWatchService(
		client,
		blockchain,
	)

	err := service.SyncLatestBlock(
		context.Background(),
	)

	if err != nil {
		t.Fatalf(
			"expected sync to succeed, got %v",
			err,
		)
	}

	if blockchain.BlockCount() != 1 {
		t.Errorf(
			"expected 1 block, got %d",
			blockchain.BlockCount(),
		)
	}

	if blockchain.BalanceOf("0xAlice") != 90 {
		t.Errorf(
			"expected Alice balance to be 90, got %d",
			blockchain.BalanceOf("0xAlice"),
		)
	}

	if blockchain.BalanceOf("0xBob") != 60 {
		t.Errorf(
			"expected Bob balance to be 60, got %d",
			blockchain.BalanceOf("0xBob"),
		)
	}
}

func TestChainWatchServiceRPCFailure(t *testing.T) {
	blockchain := newTestBlockchain()

	rpcErr := errors.New("rpc unavailable")

	client := MockBlockchainClient{
		Err: rpcErr,
	}

	service := NewChainWatchService(
		client,
		blockchain,
	)

	err := service.SyncLatestBlock(
		context.Background(),
	)

	if !errors.Is(err, rpcErr) {
		t.Fatalf(
			"expected rpc error, got %v",
			err,
		)
	}

	if blockchain.BlockCount() != 0 {
		t.Errorf(
			"expected no blocks to be processed, got %d",
			blockchain.BlockCount(),
		)
	}
}

func TestChainWatchServiceTimeout(t *testing.T) {
	blockchain := newTestBlockchain()

	client := MockBlockchainClient{
		Delay: 200 * time.Millisecond,
	}

	service := NewChainWatchService(
		client,
		blockchain,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Millisecond,
	)
	defer cancel()

	err := service.SyncLatestBlock(ctx)

	if !errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		t.Fatalf(
			"expected context deadline exceeded, got %v",
			err,
		)
	}

	if blockchain.BlockCount() != 0 {
		t.Errorf(
			"expected no blocks to be processed, got %d",
			blockchain.BlockCount(),
		)
	}
}

func TestChainWatchServiceCancellation(t *testing.T) {
	blockchain := newTestBlockchain()

	client := MockBlockchainClient{
		Delay: 200 * time.Millisecond,
	}

	service := NewChainWatchService(
		client,
		blockchain,
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	err := service.SyncLatestBlock(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context canceled, got %v",
			err,
		)
	}

	if blockchain.BlockCount() != 0 {
		t.Errorf(
			"expected no blocks to be processed, got %d",
			blockchain.BlockCount(),
		)
	}
}

func TestChainWatchServiceCompletesBeforeTimeout(t *testing.T) {
	blockchain := newTestBlockchain()

	block := Block{
		Number: 1,
		Hash:   "0xblock-fast",
		Transactions: []Transaction{
			{
				Hash:     "0xtx-fast",
				From:     "0xAlice",
				To:       "0xBob",
				ValueWei: 10,
			},
		},
	}

	client := MockBlockchainClient{
		Block: block,
		Delay: 10 * time.Millisecond,
	}

	service := NewChainWatchService(
		client,
		blockchain,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancel()

	err := service.SyncLatestBlock(ctx)

	if err != nil {
		t.Fatalf(
			"expected sync to complete before timeout, got %v",
			err,
		)
	}

	if blockchain.BlockCount() != 1 {
		t.Errorf(
			"expected 1 block to be processed, got %d",
			blockchain.BlockCount(),
		)
	}
}
