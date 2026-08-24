package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	blockchain := NewBlockchain(
		map[Address]uint64{
			"0xAlice":   100,
			"0xBob":     50,
			"0xCharlie": 0,
		},
	)

	block := Block{
		Number: 19000000,
		Hash:   "0xblock001",
		Transactions: []Transaction{
			{
				Hash:     "0xtx001",
				From:     "0xAlice",
				To:       "0xBob",
				ValueWei: 10,
				GasUsed:  21000,
			},
			{
				Hash:     "0xtx002",
				From:     "0xBob",
				To:       "0xCharlie",
				ValueWei: 20,
				GasUsed:  21000,
			},
		},
		Timestamp: 1750000000,
	}

	client := &MockBlockchainClient{
		Block: block,
	}

	service := NewChainWatchService(
		client,
		blockchain,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	fmt.Println("Before sync:")
	blockchain.PrintBalances()
	fmt.Println()

	err := service.SyncLatestBlock(ctx)

	if err != nil {
		fmt.Println("Sync failed:", err)
		return
	}

	fmt.Println("Sync successful")
	fmt.Println()

	fmt.Println("After sync:")
	blockchain.PrintBalances()

	fmt.Println()
	fmt.Println(
		"Total blocks:",
		blockchain.BlockCount(),
	)
}
