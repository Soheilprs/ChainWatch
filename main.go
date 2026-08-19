package main

import (
	"errors"
	"fmt"
)

func main() {
	blockchain := Blockchain{
		Blocks: []Block{},

		Balances: map[string]uint64{"0xAlice": 100,
			"0xBob":     50,
			"0xCharlie": 0},
		ProcessedTransactions: map[string]bool{},
	}

	tx1 := Transaction{
		Hash:     "0xtx001",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
		GasUsed:  21000,
	}

	tx2 := Transaction{
		Hash:     "0xtx002",
		From:     "0xBob",
		To:       "0xCharlie",
		ValueWei: 20,
		GasUsed:  21000,
	}

	block := Block{
		Number: 19000000,
		Hash:   "0xblock001",
		Transactions: []Transaction{
			tx1,
			tx2,
		},
		Timestamp: 1750000000,
	}

	fmt.Println("Before block:")
	blockchain.PrintBalances()

	fmt.Println()

	err := blockchain.ProcessBlock(block)

	if err != nil {
		fmt.Println("Block failed:", err)
		return
	}

	fmt.Println("Block processed successfully")

	fmt.Println()

	fmt.Println("After block:")
	blockchain.PrintBalances()

	fmt.Println()

	fmt.Println("Total blocks:", len(blockchain.Blocks))

	tx3 := Transaction{
		Hash:     "0xfail123",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 999999,
	}

	block2 := Block{
		Number: 19000001,
		Hash:   "0xblock002",
		Transactions: []Transaction{
			tx3,
		},
		Timestamp: 1750000000,
	}

	err2 := blockchain.ProcessBlock(block2)

	fmt.Println(err2)

	fmt.Println(
		errors.Is(err2, ErrInsufficientBalance),
	)
}
