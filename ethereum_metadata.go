package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var ErrInvalidTokenAddress = errors.New(
	"invalid token address",
)

const erc20MetadataABIJSON = `
[
	{
		"inputs": [],
		"name": "name",
		"outputs": [
			{
				"internalType": "string",
				"name": "",
				"type": "string"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "symbol",
		"outputs": [
			{
				"internalType": "string",
				"name": "",
				"type": "string"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "decimals",
		"outputs": [
			{
				"internalType": "uint8",
				"name": "",
				"type": "uint8"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]
`

var erc20MetadataABI = mustParseERC20MetadataABI()

var _ TokenMetadataFetcher = (*EthereumClient)(nil)

func mustParseERC20MetadataABI() abi.ABI {
	parsed, err :=
		abi.JSON(
			strings.NewReader(
				erc20MetadataABIJSON,
			),
		)

	if err != nil {
		panic(
			fmt.Sprintf(
				"failed to parse ERC20 metadata ABI: %v",
				err,
			),
		)
	}

	return parsed
}

func (e *EthereumClient) FetchTokenMetadata(
	ctx context.Context,
	address Address,
) (TokenMetadata, error) {
	if !common.IsHexAddress(
		string(address),
	) {
		return TokenMetadata{},
			ErrInvalidTokenAddress
	}

	tokenAddress :=
		common.HexToAddress(
			string(address),
		)

	name, err :=
		e.callERC20StringMethod(
			ctx,
			tokenAddress,
			"name",
		)

	if err != nil {
		return TokenMetadata{},
			fmt.Errorf(
				"failed to fetch token name: %w",
				err,
			)
	}

	symbol, err :=
		e.callERC20StringMethod(
			ctx,
			tokenAddress,
			"symbol",
		)

	if err != nil {
		return TokenMetadata{},
			fmt.Errorf(
				"failed to fetch token symbol: %w",
				err,
			)
	}

	decimals, err :=
		e.callERC20Decimals(
			ctx,
			tokenAddress,
		)

	if err != nil {
		return TokenMetadata{},
			fmt.Errorf(
				"failed to fetch token decimals: %w",
				err,
			)
	}

	return TokenMetadata{
		Address: Address(
			tokenAddress.Hex(),
		),

		Name: name,

		Symbol: symbol,

		Decimals: decimals,
	}, nil
}

func (e *EthereumClient) callERC20StringMethod(
	ctx context.Context,
	address common.Address,
	method string,
) (string, error) {
	data, err :=
		erc20MetadataABI.Pack(
			method,
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"failed to encode %s call: %w",
				method,
				err,
			)
	}

	result, err :=
		e.client.CallContract(
			ctx,
			ethereum.CallMsg{
				To:   &address,
				Data: data,
			},
			nil,
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"%s eth_call failed: %w",
				method,
				err,
			)
	}

	values, err :=
		erc20MetadataABI.Unpack(
			method,
			result,
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"failed to decode %s result: %w",
				method,
				err,
			)
	}

	if len(values) != 1 {
		return "",
			fmt.Errorf(
				"expected one %s result, got %d",
				method,
				len(values),
			)
	}

	value, ok :=
		values[0].(string)

	if !ok {
		return "",
			fmt.Errorf(
				"unexpected %s result type",
				method,
			)
	}

	return value, nil
}

func (e *EthereumClient) callERC20Decimals(
	ctx context.Context,
	address common.Address,
) (uint8, error) {
	data, err :=
		erc20MetadataABI.Pack(
			"decimals",
		)

	if err != nil {
		return 0,
			fmt.Errorf(
				"failed to encode decimals call: %w",
				err,
			)
	}

	result, err :=
		e.client.CallContract(
			ctx,
			ethereum.CallMsg{
				To:   &address,
				Data: data,
			},
			nil,
		)

	if err != nil {
		return 0,
			fmt.Errorf(
				"decimals eth_call failed: %w",
				err,
			)
	}

	values, err :=
		erc20MetadataABI.Unpack(
			"decimals",
			result,
		)

	if err != nil {
		return 0,
			fmt.Errorf(
				"failed to decode decimals result: %w",
				err,
			)
	}

	if len(values) != 1 {
		return 0,
			fmt.Errorf(
				"expected one decimals result, got %d",
				len(values),
			)
	}

	decimals, ok :=
		values[0].(uint8)

	if !ok {
		return 0,
			errors.New(
				"unexpected decimals result type",
			)
	}

	return decimals, nil
}
