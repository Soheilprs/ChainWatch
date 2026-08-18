package main

import "fmt"

func main() {
	blockchain := Blockchain{
		Balances: map[string]uint64{
			"0xAlice":   100,
			"0xBob":     50,
			"0xCharlie": 0,
		},
		ProcessedTransactions: map[string]bool{},
	}

	tx := Transaction{
		Hash:     "0xtx001",
		From:     "0xAlice",
		To:       "0xBob",
		ValueWei: 10,
		GasUsed:  21000,
	}

	fmt.Println("ChainWatch started")
	fmt.Println()

	fmt.Println("Before transaction:")
	blockchain.PrintBalances()

	fmt.Println()

	err := blockchain.ProcessTransaction(tx)

	if err != nil {
		fmt.Println("Transaction failed:", err)
	} else {
		fmt.Println("Transaction processed successfully")
	}

	fmt.Println()
	fmt.Println("After transaction:")
	blockchain.PrintBalances()
}
