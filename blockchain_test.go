package main

import (
	"errors"
	"testing"
)

func newTestBlockchain() *Blockchain {
	return NewBlockchain(
		map[Address]uint64{
			"0xAlice":   100,
			"0xBob":     50,
			"0xCharlie": 0,
		},
	)
}

func TestProcessTransactionSuccess(t *testing.T) {
	blockchain := newTestBlockchain()

	tx := Transaction{
		Hash:     "0xtx001",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
	}

	err := blockchain.ProcessTransaction(tx)

	if err != nil {
		t.Fatalf("expected transaction to succeed, got error: %v", err)
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

	if !blockchain.IsTransactionProcessed(tx.Hash) {
		t.Error("expected transaction to be marked as processed")
	}
}

func TestProcessTransactionInsufficientBalance(t *testing.T) {
	blockchain := newTestBlockchain()

	tx := Transaction{
		Hash:     "0xtx002",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 1000,
	}

	err := blockchain.ProcessTransaction(tx)

	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf(
			"expected ErrInsufficientBalance, got %v",
			err,
		)
	}

	if blockchain.BalanceOf("0xAlice") != 100 {
		t.Errorf(
			"expected Alice balance to remain 100, got %d",
			blockchain.BalanceOf("0xAlice"),
		)
	}

	if blockchain.BalanceOf("0xBob") != 50 {
		t.Errorf(
			"expected Bob balance to remain 50, got %d",
			blockchain.BalanceOf("0xBob"),
		)
	}

	if blockchain.IsTransactionProcessed(tx.Hash) {
		t.Error("failed transaction should not be marked as processed")
	}
}

func TestProcessTransactionUnknownSender(t *testing.T) {
	blockchain := newTestBlockchain()

	tx := Transaction{
		Hash:     "0xtx003",
		From:     "0xUnknown",
		To:       "0xBob",
		ValueWei: 10,
	}

	err := blockchain.ProcessTransaction(tx)

	if !errors.Is(err, ErrSenderNotFound) {
		t.Fatalf(
			"expected ErrSenderNotFound, got %v",
			err,
		)
	}

	if blockchain.BalanceOf("0xBob") != 50 {
		t.Errorf(
			"expected Bob balance to remain 50, got %d",
			blockchain.BalanceOf("0xBob"),
		)
	}
}

func TestProcessTransactionDuplicate(t *testing.T) {
	blockchain := newTestBlockchain()

	tx := Transaction{
		Hash:     "0xtx004",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
	}

	err := blockchain.ProcessTransaction(tx)

	if err != nil {
		t.Fatalf("first transaction should succeed: %v", err)
	}

	err = blockchain.ProcessTransaction(tx)

	if !errors.Is(err, ErrTransactionAlreadyProcessed) {
		t.Fatalf(
			"expected ErrTransactionAlreadyProcessed, got %v",
			err,
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

func TestProcessTransactionNewReceiver(t *testing.T) {
	blockchain := newTestBlockchain()

	tx := Transaction{
		Hash:     "0xtx006",
		From:     "0xAlice",
		To:       "0xDave",
		ValueWei: 20,
	}

	err := blockchain.ProcessTransaction(tx)

	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	if blockchain.BalanceOf("0xAlice") != 80 {
		t.Errorf(
			"expected Alice balance to be 80, got %d", blockchain.BalanceOf("0xAlice"),
		)
	}

	if blockchain.BalanceOf("0xDave") != 20 {

		t.Errorf(
			"expected Dave balance to be 20, got %d",
			blockchain.BalanceOf("0xDave"),
		)
	}

	if !blockchain.IsTransactionProcessed(tx.Hash) {
		t.Error("expected transaction to be marked as processed")
	}
}

func TestProcessBlockAtomicFailure(t *testing.T) {
	blockchain := newTestBlockchain()

	tx1 := Transaction{
		Hash:     "0xtx101",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
	}

	tx2 := Transaction{
		Hash:     "0xtx102",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 500,
	}

	block := Block{
		Number: 1,
		Hash:   "0xblock001",
		Transactions: []Transaction{
			tx1,
			tx2,
		},
	}

	err := blockchain.ProcessBlock(block)

	if err == nil {
		t.Fatal("expected block processing to fail")
	}

	if blockchain.BalanceOf("0xAlice") != 100 {
		t.Errorf(
			"expected Alice balance to remain 100, got %d",
			blockchain.BalanceOf("0xAlice"),
		)
	}

	if blockchain.BalanceOf("0xBob") != 50 {
		t.Errorf(
			"expected Bob balance to remain 50, got %d",
			blockchain.BalanceOf("0xBob"),
		)
	}

	if blockchain.BlockCount() != 0 {
		t.Errorf(
			"expected no blocks to be appended, got %d",
			blockchain.BlockCount(),
		)
	}

	if blockchain.IsTransactionProcessed(tx1.Hash) {
		t.Error("tx1 should not be marked as processed after block rollback")
	}
}

func TestProcessBlockSuccess(t *testing.T) {
	blockchain := newTestBlockchain()

	tx1 := Transaction{
		Hash:     "0xtx201",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
	}

	tx2 := Transaction{
		Hash:     "0xtx202",
		From:     "0xBob",
		To:       "0xCharlie",
		ValueWei: 20,
	}

	block := Block{
		Number: 1,
		Hash:   "0xblock-success",
		Transactions: []Transaction{
			tx1,
			tx2,
		},
	}

	err := blockchain.ProcessBlock(block)

	if err != nil {
		t.Fatalf("expected block to succeed, got error: %v", err)
	}

	if blockchain.BalanceOf("0xAlice") != 90 {
		t.Errorf(
			"expected Alice balance to be 90, got %d",
			blockchain.BalanceOf("0xAlice"),
		)
	}

	if blockchain.BalanceOf("0xBob") != 40 {
		t.Errorf(
			"expected Bob balance to be 40, got %d",
			blockchain.BalanceOf("0xBob"),
		)
	}

	if blockchain.BalanceOf("0xCharlie") != 20 {
		t.Errorf(
			"expected Charlie balance to be 20, got %d",
			blockchain.BalanceOf("0xCharlie"),
		)
	}

	if blockchain.BlockCount() != 1 {
		t.Errorf(
			"expected 1 block, got %d",
			blockchain.BlockCount(),
		)
	}

	if !blockchain.IsTransactionProcessed(tx1.Hash) {
		t.Error("expected tx1 to be marked as processed")
	}

	if !blockchain.IsTransactionProcessed(tx2.Hash) {
		t.Error("expected tx2 to be marked as processed")
	}
}

func TestTransactionValidationEmptyHash(t *testing.T) {
	tx := Transaction{
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
	}

	err := tx.Validate()

	if !errors.Is(err, ErrEmptyTransactionHash) {
		t.Fatalf("expected ErrEmptyTransactionHash, got %v", err)
	}
}

func TestTransactionValidationZeroValue(t *testing.T) {
	tx := Transaction{
		Hash:     "0xtx001",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 0,
	}

	err := tx.Validate()

	if !errors.Is(err, ErrZeroValue) {
		t.Fatalf(
			"expected ErrZeroValue, got %v",
			err,
		)
	}
}

func TestNewBlockchainCopiesInitialBalances(t *testing.T) {
	initialBalances := map[Address]uint64{
		"0xAlice": 100,
	}

	blockchain := NewBlockchain(initialBalances)

	initialBalances["0xAlice"] = 999

	if blockchain.BalanceOf("0xAlice") != 100 {
		t.Errorf(
			"expected blockchain balance to remain 100, got %d",
			blockchain.BalanceOf("0xAlice"),
		)
	}
}

func TestTransactionValidationEmptySender(t *testing.T) {
	tx := Transaction{
		Hash:     "0xabc123",
		To:       "0xAlice",
		ValueWei: 10,
	}

	err := tx.Validate()

	if !errors.Is(err, ErrEmptySender) {
		t.Errorf("expected ErrEmptySender, got %v", err)
	}
}

func TestTransactionValidationEmptyReceiver(t *testing.T) {
	tx := Transaction{
		Hash:     "0xabc123",
		From:     "0xBob",
		ValueWei: 10,
	}

	err := tx.Validate()

	if !errors.Is(err, ErrEmptyReceiver) {
		t.Errorf("expected ErrEmptyReceiver, got %v", err)
	}
}
