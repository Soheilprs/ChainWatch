package main

import (
	"errors"
	"fmt"
)

type Address string
type TransactionHash string
type BlockHash string

var (
	ErrTransactionAlreadyProcessed = errors.New("transaction already processed")
	ErrSenderNotFound              = errors.New("sender does not exist")
	ErrInsufficientBalance         = errors.New("insufficient balance")

	ErrEmptyTransactionHash = errors.New("transaction hash is empty")
	ErrEmptySender          = errors.New("sender is empty")
	ErrEmptyReceiver        = errors.New("receiver is empty")
	ErrZeroValue            = errors.New("transaction value must be greater than zero")
)

type Block struct {
	Number       uint64
	Hash         BlockHash
	Transactions []Transaction
	Timestamp    uint64
}

type Blockchain struct {
	blocks                []Block
	balances              map[Address]uint64
	processedTransactions map[TransactionHash]bool
}

type Transaction struct {
	Hash     TransactionHash
	From     Address
	To       Address
	ValueWei uint64
	GasUsed  uint64
}

func NewBlockchain(
	initialBalances map[Address]uint64,
) *Blockchain {
	return &Blockchain{
		blocks:                []Block{},
		balances:              copyBalances(initialBalances),
		processedTransactions: map[TransactionHash]bool{},
	}
}

func (tx Transaction) Validate() error {
	if tx.Hash == "" {
		return ErrEmptyTransactionHash
	}

	if tx.From == "" {
		return ErrEmptySender
	}

	if tx.To == "" {
		return ErrEmptyReceiver
	}

	if tx.ValueWei == 0 {
		return ErrZeroValue
	}

	return nil
}

func (bc Blockchain) PrintBalances() {
	fmt.Println("Wallet Balances:")

	for address, balance := range bc.balances {
		fmt.Println(address, ":", balance)
	}
}

func (bc *Blockchain) BalanceOf(address Address) uint64 {
	return bc.balances[address]
}

func (bc *Blockchain) IsTransactionProcessed(
	hash TransactionHash,
) bool {
	return bc.processedTransactions[hash]
}

func (bc *Blockchain) BlockCount() int {
	return len(bc.blocks)
}

func copyBalances(
	balances map[Address]uint64,
) map[Address]uint64 {
	copied := make(map[Address]uint64)

	for address, balance := range balances {
		copied[address] = balance
	}

	return copied
}

func copyProcessedTransactions(
	processed map[TransactionHash]bool,
) map[TransactionHash]bool {
	copied := make(map[TransactionHash]bool)

	for hash, value := range processed {
		copied[hash] = value
	}

	return copied
}

func (bc *Blockchain) ProcessTransaction(tx Transaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}

	if bc.processedTransactions[tx.Hash] {
		return ErrTransactionAlreadyProcessed
	}

	senderBalance, exists := bc.balances[tx.From]

	if !exists {
		return ErrSenderNotFound
	}

	if senderBalance < tx.ValueWei {
		return ErrInsufficientBalance
	}

	bc.balances[tx.From] -= tx.ValueWei
	bc.balances[tx.To] += tx.ValueWei

	bc.processedTransactions[tx.Hash] = true

	return nil
}

func (bc *Blockchain) ProcessBlock(block Block) error {
	tempBlockchain := Blockchain{
		balances: copyBalances(
			bc.balances,
		),
		processedTransactions: copyProcessedTransactions(
			bc.processedTransactions,
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

	bc.balances = tempBlockchain.balances
	bc.processedTransactions = tempBlockchain.processedTransactions
	bc.blocks = append(bc.blocks, block)

	return nil
}
