package main

import (
	"errors"
	"fmt"
)

var (
	ErrTransactionAlreadyProcessed = errors.New("transaction already processed")
	ErrSenderNotFound              = errors.New("sender does not exist")
	ErrInsufficientBalance         = errors.New("insufficient balance")
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

func copyBalances(
	balances map[string]uint64,
) map[string]uint64 {
	copied := make(map[string]uint64)

	for address, balance := range balances {
		copied[address] = balance
	}

	return copied
}

func copyProcessedTransactions(
	processed map[string]bool,
) map[string]bool {
	copied := make(map[string]bool)

	for hash, value := range processed {
		copied[hash] = value
	}

	return copied
}

func (bc *Blockchain) ProcessTransaction(tx Transaction) error {
	if bc.ProcessedTransactions[tx.Hash] {
		return ErrTransactionAlreadyProcessed
	}

	senderBalance, exists := bc.Balances[tx.From]

	if !exists {
		return ErrSenderNotFound
	}

	if senderBalance < tx.ValueWei {
		return ErrInsufficientBalance
	}

	bc.Balances[tx.From] -= tx.ValueWei
	bc.Balances[tx.To] += tx.ValueWei

	bc.ProcessedTransactions[tx.Hash] = true

	return nil
}

func (bc *Blockchain) ProcessBlock(block Block) error {
	tempBlockchain := Blockchain{
		Balances: copyBalances(
			bc.Balances,
		),
		ProcessedTransactions: copyProcessedTransactions(
			bc.ProcessedTransactions,
		),
	}

	for _, tx := range block.Transactions {
		err := tempBlockchain.ProcessTransaction(tx)

		if err != nil {
			return fmt.Errorf(
				"failed to process transaction %s: %w",
				tx.Hash,
				err,
			)
		}
	}

	bc.Balances = tempBlockchain.Balances
	bc.ProcessedTransactions = tempBlockchain.ProcessedTransactions
	bc.Blocks = append(bc.Blocks, block)

	return nil
}
