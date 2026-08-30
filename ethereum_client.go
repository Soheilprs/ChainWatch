package main

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

	return e.observedBlockFromEthereumBlock(
		ctx,
		block,
	)
}

func observedLogFromEthereumLog(
	log types.Log,
) ObservedLog {
	topics := make(
		[]string,
		0,
		len(log.Topics),
	)

	for _, topic := range log.Topics {
		topics = append(
			topics,
			topic.Hex(),
		)
	}

	return ObservedLog{
		Address: Address(
			log.Address.Hex(),
		),
		Topics: topics,
		Data: append(
			[]byte(nil),
			log.Data...,
		),
		Index: log.Index,
		TransactionHash: TransactionHash(
			log.TxHash.Hex(),
		),
		BlockNumber: log.BlockNumber,
		Removed:     log.Removed,
	}
}

func (e *EthereumClient) GetTransactionReceipt(
	ctx context.Context,
	hash TransactionHash,
) (ObservedReceipt, error) {
	receipt, err := e.client.TransactionReceipt(
		ctx,
		common.HexToHash(string(hash)),
	)

	if err != nil {
		return ObservedReceipt{}, fmt.Errorf(
			"failed to fetch receipt for transaction %s: %w",
			hash,
			err,
		)
	}

	logs := make(
		[]ObservedLog,
		0,
		len(receipt.Logs),
	)

	for _, log := range receipt.Logs {
		logs = append(
			logs,
			observedLogFromEthereumLog(
				*log,
			),
		)
	}

	var contractAddress *Address

	if receipt.ContractAddress != (common.Address{}) {
		address := Address(
			receipt.ContractAddress.Hex(),
		)

		contractAddress = &address
	}

	var effectiveGasPrice *big.Int

	if receipt.EffectiveGasPrice != nil {
		effectiveGasPrice = new(big.Int).Set(
			receipt.EffectiveGasPrice,
		)
	}

	return ObservedReceipt{
		TransactionHash: TransactionHash(
			receipt.TxHash.Hex(),
		),
		Status:            receipt.Status,
		GasUsed:           receipt.GasUsed,
		EffectiveGasPrice: effectiveGasPrice,
		ContractAddress:   contractAddress,
		Logs:              logs,
	}, nil
}

func (e *EthereumClient) GetERC20TransfersByBlock(
	ctx context.Context,
	block ObservedBlock,
) (BlockTransferIndex, error) {
	blockHash := common.HexToHash(
		string(block.Hash),
	)

	logs, err := e.client.FilterLogs(
		ctx,
		ethereum.FilterQuery{
			BlockHash: &blockHash,
			Topics: [][]common.Hash{
				{
					erc20TransferTopic,
				},
			},
		},
	)

	if err != nil {
		return BlockTransferIndex{}, fmt.Errorf(
			"failed to fetch ERC20 transfer logs for block %s: %w",
			block.Hash,
			err,
		)
	}

	transfers := make(
		[]ERC20Transfer,
		0,
		len(logs),
	)

	for _, ethereumLog := range logs {
		log := observedLogFromEthereumLog(
			ethereumLog,
		)

		if !IsERC20TransferLog(log) {
			continue
		}

		transfer, err := DecodeERC20Transfer(
			log,
			log.TransactionHash,
		)

		if err != nil {
			return BlockTransferIndex{}, fmt.Errorf(
				"failed to decode ERC20 transfer log %d: %w",
				log.Index,
				err,
			)
		}

		transfers = append(
			transfers,
			transfer,
		)
	}

	return BlockTransferIndex{
		BlockNumber: block.Number,
		BlockHash:   block.Hash,
		Transfers:   transfers,
	}, nil
}

func (e *EthereumClient) Close() {
	e.client.Close()
}

func (e *EthereumClient) observedBlockFromEthereumBlock(
	ctx context.Context,
	block *types.Block,
) (ObservedBlock, error) {
	transactions := make(
		[]ObservedTransaction,
		0,
		len(block.Transactions()),
	)

	for index, tx := range block.Transactions() {
		sender, err :=
			e.client.TransactionSender(
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

		transactions = append(
			transactions,
			ObservedTransaction{
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
			},
		)
	}

	return ObservedBlock{
		Number: block.NumberU64(),
		Hash: BlockHash(
			block.Hash().Hex(),
		),
		Timestamp:    block.Time(),
		Transactions: transactions,
	}, nil
}

func (e *EthereumClient) GetObservedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (ObservedBlock, error) {
	number := new(big.Int).SetUint64(
		blockNumber,
	)

	block, err := e.client.BlockByNumber(
		ctx,
		number,
	)

	if err != nil {
		return ObservedBlock{}, fmt.Errorf(
			"failed to fetch ethereum block %d: %w",
			blockNumber,
			err,
		)
	}

	return e.observedBlockFromEthereumBlock(
		ctx,
		block,
	)
}
