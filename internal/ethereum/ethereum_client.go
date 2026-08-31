package ethereum

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/soheilprs/chainwatch/internal/domain"
)

type EthereumClient struct {
	client *ethclient.Client
}

func NewEthereumClient(
	ctx context.Context,
	rpcURL string,
) (*EthereumClient, error) {
	client, err := ethclient.DialContext(
		ctx,
		rpcURL,
	)

	if err != nil {
		return nil, domain.NewDomainError(
			domain.ErrRPC,
			"connect to Ethereum RPC",
			err,
		)
	}

	return &EthereumClient{
		client: client,
	}, nil
}

func (e *EthereumClient) GetLatestBlock(
	ctx context.Context,
) (result domain.Block, err error) {
	defer domain.ClassifyError(&err, domain.ErrRPC, "fetch latest Ethereum block")

	header, err := e.client.HeaderByNumber(
		ctx,
		nil,
	)

	if err != nil {
		return domain.Block{}, fmt.Errorf(
			"failed to fetch latest ethereum header: %w",
			err,
		)
	}

	return domain.Block{
		Number:       header.Number.Uint64(),
		Hash:         domain.BlockHash(header.Hash().Hex()),
		Timestamp:    header.Time,
		Transactions: []domain.Transaction{},
	}, nil
}

func (e *EthereumClient) GetLatestObservedBlock(
	ctx context.Context,
) (result domain.ObservedBlock, err error) {
	defer domain.ClassifyError(&err, domain.ErrRPC, "fetch latest observed Ethereum block")

	block, err := e.client.BlockByNumber(
		ctx,
		nil,
	)

	if err != nil {
		return domain.ObservedBlock{}, fmt.Errorf(
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
) domain.ObservedLog {
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

	return domain.ObservedLog{
		Address: domain.Address(
			log.Address.Hex(),
		),
		Topics: topics,
		Data: append(
			[]byte(nil),
			log.Data...,
		),
		Index: log.Index,
		TransactionHash: domain.TransactionHash(
			log.TxHash.Hex(),
		),
		BlockNumber: log.BlockNumber,
		Removed:     log.Removed,
	}
}

func (e *EthereumClient) GetTransactionReceipt(
	ctx context.Context,
	hash domain.TransactionHash,
) (result domain.ObservedReceipt, err error) {
	defer domain.ClassifyError(&err, domain.ErrRPC, "fetch Ethereum transaction receipt")

	receipt, err := e.client.TransactionReceipt(
		ctx,
		common.HexToHash(string(hash)),
	)

	if err != nil {
		return domain.ObservedReceipt{}, fmt.Errorf(
			"failed to fetch receipt for transaction %s: %w",
			hash,
			err,
		)
	}

	logs := make(
		[]domain.ObservedLog,
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

	var contractAddress *domain.Address

	if receipt.ContractAddress != (common.Address{}) {
		address := domain.Address(
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

	return domain.ObservedReceipt{
		TransactionHash: domain.TransactionHash(
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
	block domain.ObservedBlock,
) (result domain.BlockTransferIndex, err error) {
	defer domain.ClassifyError(&err, domain.ErrRPC, "fetch ERC20 transfer logs")

	blockHash := common.HexToHash(
		string(block.Hash),
	)

	logs, err := e.client.FilterLogs(
		ctx,
		ethereum.FilterQuery{
			BlockHash: &blockHash,
			Topics: [][]common.Hash{
				{
					domain.ERC20TransferTopic(),
				},
			},
		},
	)

	if err != nil {
		return domain.BlockTransferIndex{}, fmt.Errorf(
			"failed to fetch ERC20 transfer logs for block %s: %w",
			block.Hash,
			err,
		)
	}

	transfers := make(
		[]domain.ERC20Transfer,
		0,
		len(logs),
	)

	for _, ethereumLog := range logs {
		log := observedLogFromEthereumLog(
			ethereumLog,
		)

		if !domain.IsERC20TransferLog(log) {
			continue
		}

		transfer, err := domain.DecodeERC20Transfer(
			log,
			log.TransactionHash,
		)

		if err != nil {
			return domain.BlockTransferIndex{}, fmt.Errorf(
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

	return domain.BlockTransferIndex{
		BlockNumber: block.Number,
		BlockHash:   block.Hash,
		ParentHash:  block.ParentHash,
		Transfers:   transfers,
	}, nil
}

func (e *EthereumClient) Close() {
	e.client.Close()
}

func (e *EthereumClient) observedBlockFromEthereumBlock(
	ctx context.Context,
	block *types.Block,
) (domain.ObservedBlock, error) {
	transactions := make(
		[]domain.ObservedTransaction,
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
			return domain.ObservedBlock{}, fmt.Errorf(
				"failed to recover sender for transaction %s: %w",
				tx.Hash().Hex(),
				err,
			)
		}

		var to *domain.Address

		if tx.To() != nil {
			address := domain.Address(
				tx.To().Hex(),
			)

			to = &address
		}

		transactions = append(
			transactions,
			domain.ObservedTransaction{
				Hash: domain.TransactionHash(
					tx.Hash().Hex(),
				),
				From: domain.Address(
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

	return domain.ObservedBlock{
		Number: block.NumberU64(),
		Hash: domain.BlockHash(
			block.Hash().Hex(),
		),
		ParentHash: domain.BlockHash(
			block.ParentHash().Hex(),
		),
		Timestamp:    block.Time(),
		Transactions: transactions,
	}, nil
}

func (e *EthereumClient) GetObservedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (result domain.ObservedBlock, err error) {
	defer domain.ClassifyError(&err, domain.ErrRPC, "fetch Ethereum block by number")

	number := new(big.Int).SetUint64(
		blockNumber,
	)

	block, err := e.client.BlockByNumber(
		ctx,
		number,
	)

	if err != nil {
		return domain.ObservedBlock{}, fmt.Errorf(
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
