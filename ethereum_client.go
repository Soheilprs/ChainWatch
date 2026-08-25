package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

type EthereumClient struct {
	client *ethclient.Client
}

var _ BlockchainClient = (*EthereumClient)(nil)

func NewEthereumClient(
	ctx context.Context,
	rpcURL string,
) (*EthereumClient, error) {
	client, err := ethclient.DialContext(
		ctx,
		rpcURL,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"failed to connect to ethereum rpc: %w",
			err,
		)
	}

	return &EthereumClient{
		client: client,
	}, nil
}

func (e *EthereumClient) GetLatestBlock(
	ctx context.Context,
) (Block, error) {
	header, err := e.client.HeaderByNumber(
		ctx,
		nil,
	)

	if err != nil {
		return Block{}, fmt.Errorf(
			"failed to fetch latest ethereum header: %w",
			err,
		)
	}

	return Block{
		Number:       header.Number.Uint64(),
		Hash:         BlockHash(header.Hash().Hex()),
		Timestamp:    header.Time,
		Transactions: []Transaction{},
	}, nil
}

func (e *EthereumClient) GetLatestObservedBlock(
	ctx context.Context,
) (ObservedBlock, error) {
	block, err := e.client.BlockByNumber(
		ctx,
		nil,
	)

	if err != nil {
		return ObservedBlock{}, fmt.Errorf(
			"failed to fetch latest ethereum block: %w",
			err,
		)
	}

	transactions := make(
		[]ObservedTransaction,
		0,
		len(block.Transactions()),
	)

	for index, tx := range block.Transactions() {
		sender, err := e.client.TransactionSender(
			ctx,
			tx,
			block.Hash(),
			uint(index),
		)

		if err != nil {
			return ObservedBlock{}, fmt.Errorf(
				"failed to recover sender for transaction %s: %w",
				tx.Hash().Hex(),
				err,
			)
		}

		var to *Address

		if tx.To() != nil {
			address := Address(
				tx.To().Hex(),
			)

			to = &address
		}

		observedTx := ObservedTransaction{
			Hash: TransactionHash(
				tx.Hash().Hex(),
			),
			From: Address(
				sender.Hex(),
			),
			To: to,
			ValueWei: new(big.Int).Set(
				tx.Value(),
			),
			Nonce:    tx.Nonce(),
			GasLimit: tx.Gas(),
			Type:     tx.Type(),
		}

		transactions = append(
			transactions,
			observedTx,
		)
	}

	return ObservedBlock{
		Number:       block.NumberU64(),
		Hash:         BlockHash(block.Hash().Hex()),
		Timestamp:    block.Time(),
		Transactions: transactions,
	}, nil
}

func (e *EthereumClient) Close() {
	e.client.Close()
}
