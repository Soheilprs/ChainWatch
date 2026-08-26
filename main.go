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

		receipt, err :=
			client.GetTransactionReceipt(
				ctx,
				tx.Hash,
			)

		if err != nil {
			fmt.Println(
				"Receipt error:",
				err,
			)
			continue
		}

		fmt.Println(
			"Gas used:",
			receipt.GasUsed,
		)

		fmt.Println(
			"Successful:",
			receipt.Successful(),
		)

		fmt.Println(
			"Log count:",
			receipt.LogCount(),
		)

		if receipt.EffectiveGasPrice != nil {
			fmt.Println(
				"Effective gas price:",
				receipt.EffectiveGasPrice.String(),
			)

			fmt.Println(
				"Transaction fee Wei:",
				receipt.FeeWei().String(),
			)
		}

		if receipt.ContractAddress != nil {
			fmt.Println(
				"Created contract:",
				*receipt.ContractAddress,
			)
		}

		for _, log := range receipt.Logs {
			if !IsERC20TransferLog(log) {
				continue
			}

			transfer, err :=
				DecodeERC20Transfer(
					log,
					tx.Hash,
				)

			if err != nil {
				fmt.Println(
					"Transfer decode error:",
					err,
				)
				continue
			}

			fmt.Println()
			fmt.Println(
				"  ERC-20 Transfer",
			)
			fmt.Println(
				"  Token:",
				transfer.Token,
			)
			fmt.Println(
				"  From:",
				transfer.From,
			)
			fmt.Println(
				"  To:",
				transfer.To,
			)
			fmt.Println(
				"  Amount:",
				transfer.Value.String(),
			)
		}
	}

	transferIndex, err :=
		client.GetERC20TransfersByBlock(
			ctx,
			observedBlock,
		)

	if err != nil {
		fmt.Println(
			"Failed to index ERC20 transfers:",
			err,
		)
		return
	}

	fmt.Println()
	fmt.Println("ERC-20 Block Index")
	fmt.Println(
		"Block:",
		transferIndex.BlockHash,
	)
	fmt.Println(
		"Transfer count:",
		transferIndex.TransferCount(),
	)

	transferLimit := 10

	if transferIndex.TransferCount() < transferLimit {
		transferLimit = transferIndex.TransferCount()
	}

	for i := 0; i < transferLimit; i++ {
		transfer := transferIndex.Transfers[i]

		fmt.Println()
		fmt.Println(
			"Transfer:",
			i+1,
		)
		fmt.Println(
			"Token:",
			transfer.Token,
		)
		fmt.Println(
			"From:",
			transfer.From,
		)
		fmt.Println(
			"To:",
			transfer.To,
		)
		fmt.Println(
			"Amount:",
			transfer.Value.String(),
		)
		fmt.Println(
			"Transaction:",
			transfer.TransactionHash,
		)
	}
}
