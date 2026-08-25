package main

import (
	"context"
	"fmt"

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

func (e *EthereumClient) Close() {
	e.client.Close()
}
