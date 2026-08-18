package main

import "testing"

func newTestBlockchain() Blockchain {
	return Blockchain{
		Balances: map[string]uint64{
			"0xAlice":   100,
			"0xBob":     50,
			"0xCharlie": 0,
		},
		ProcessedTransactions: map[string]bool{},
	}
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

	if blockchain.Balances["0xAlice"] != 90 {
		t.Errorf(
			"expected Alice balance to be 90, got %d",
			blockchain.Balances["0xAlice"],
		)
	}

	if blockchain.Balances["0xBob"] != 60 {
		t.Errorf(
			"expected Bob balance to be 60, got %d",
			blockchain.Balances["0xBob"],
		)
	}

	if !blockchain.ProcessedTransactions[tx.Hash] {
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

	if err == nil {
		t.Fatal("expected insufficient balance error")
	}

	if blockchain.Balances["0xAlice"] != 100 {
		t.Errorf(
			"expected Alice balance to remain 100, got %d",
			blockchain.Balances["0xAlice"],
		)
	}

	if blockchain.Balances["0xBob"] != 50 {
		t.Errorf(
			"expected Bob balance to remain 50, got %d",
			blockchain.Balances["0xBob"],
		)
	}

	if blockchain.ProcessedTransactions[tx.Hash] {
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

	if err == nil {
		t.Fatal("expected unknown sender error")
	}

	if blockchain.Balances["0xBob"] != 50 {
		t.Errorf(
			"expected Bob balance to remain 50, got %d",
			blockchain.Balances["0xBob"],
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

	if err == nil {
		t.Fatal("expected duplicate transaction to fail")
	}

	if blockchain.Balances["0xAlice"] != 90 {
		t.Errorf(
			"expected Alice balance to be 90, got %d",
			blockchain.Balances["0xAlice"],
		)
	}

	if blockchain.Balances["0xBob"] != 60 {
		t.Errorf(
			"expected Bob balance to be 60, got %d",
			blockchain.Balances["0xBob"],
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

	if blockchain.Balances["0xAlice"] != 80 {
		t.Errorf(
			"expected Alice balance to be 80, got %d", blockchain.Balances["0xAlice"],
		)
	}

	if blockchain.Balances["0xDave"] != 20 {

		t.Errorf(
			"expected Dave balance to be 20, got %d",
			blockchain.Balances["0xDave"],
		)
	}

	if !blockchain.ProcessedTransactions[tx.Hash] {
		t.Error("expected transaction to be marked as processed")
	}
}
