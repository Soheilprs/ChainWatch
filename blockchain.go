package main

import (
	"errors"
	"fmt"
)

type Block struct {
	Number       uint64
	Hash         string
	Transactions []Transaction
	Timestamp    uint64
}

type Blockchain struct {
	Blocks                []Block
	Balances              map[string]uint64
	ProcessedTransactions map[string]bool
}

type Transaction struct {
	Hash     string
	From     string
	To       string
	ValueWei uint64
	GasUsed  uint64
}

func (bc Blockchain) PrintBalances() {
	fmt.Println("Wallet Balances:")

	for address, balance := range bc.Balances {
		fmt.Println(address, ":", balance)
	}
}

func (bc *Blockchain) ProcessTransaction(tx Transaction) error {
	if bc.ProcessedTransactions[tx.Hash] {
		return errors.New("transaction already processed")
	}

	senderBalance, exists := bc.Balances[tx.From]

	if !exists {
		return errors.New("sender does not exist")
	}

	if senderBalance < tx.ValueWei {
		return errors.New("insufficient balance")
	}

	bc.Balances[tx.From] -= tx.ValueWei
	bc.Balances[tx.To] += tx.ValueWei

	bc.ProcessedTransactions[tx.Hash] = true

	return nil
}
