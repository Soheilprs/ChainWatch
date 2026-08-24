package main

import (
	"errors"
	"testing"
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

	err := service.SyncLatestBlock()

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

	err := service.SyncLatestBlock()

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
