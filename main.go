package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func main() {
	rpcURL := os.Getenv("ETH_RPC_URL")

	if rpcURL == "" {
		fmt.Println("ETH_RPC_URL is not set")
		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	client, err := NewEthereumClient(
		ctx,
		rpcURL,
	)

	if err != nil {
		fmt.Println(
			"Failed to create Ethereum client:",
			err,
		)
		return
	}

	defer client.Close()

	blockchain := NewBlockchain(
		map[Address]uint64{},
	)

	service := NewChainWatchService(
		client,
		blockchain,
	)

	err = service.SyncLatestBlock(ctx)

	if err != nil {
		fmt.Println(
			"Sync failed:",
			err,
		)
		return
	}

	block, exists := blockchain.LatestBlock()

	if !exists {
		fmt.Println("No block was processed")
		return
	}

	fmt.Println("Ethereum block synchronized")
	fmt.Println("Block number:", block.Number)
	fmt.Println("Block hash:", block.Hash)
	fmt.Println("Timestamp:", block.Timestamp)

	observedBlock, err :=
		client.GetLatestObservedBlock(ctx)

	if err != nil {
		fmt.Println(
			"Failed to fetch observed block:",
			err,
		)
		return
	}

	fmt.Println()
	fmt.Println("Real Ethereum transactions")

	fmt.Println(
		"Transaction count:",
		observedBlock.TransactionCount(),
	)

	limit := 5

	if observedBlock.TransactionCount() < limit {
		limit = observedBlock.TransactionCount()
	}

	for i := 0; i < limit; i++ {
		tx := observedBlock.Transactions[i]

		fmt.Println()
		fmt.Println("Transaction:", i+1)
		fmt.Println("Hash:", tx.Hash)
		fmt.Println("From:", tx.From)

		if tx.To == nil {
			fmt.Println(
				"To: contract creation",
			)
		} else {
			fmt.Println(
				"To:",
				*tx.To,
			)
		}

		fmt.Println(
			"Value Wei:",
			tx.ValueWei.String(),
		)

		fmt.Println(
			"Nonce:",
			tx.Nonce,
		)

		fmt.Println(
			"Gas limit:",
			tx.GasLimit,
		)

		fmt.Println(
			"Type:",
			tx.Type,
		)
	}
}
