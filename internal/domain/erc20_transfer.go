package domain

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrNotERC20Transfer = errors.New(
		"log is not an ERC20 transfer",
	)

	ErrInvalidERC20TransferData = errors.New(
		"invalid ERC20 transfer data",
	)
)

var erc20TransferTopic = crypto.Keccak256Hash(
	[]byte(
		"Transfer(address,address,uint256)",
	),
)

// ERC20TransferTopic returns the canonical event topic used by Ethereum log
// filters without exposing mutable domain state.
func ERC20TransferTopic() common.Hash {
	return erc20TransferTopic
}

type ERC20Transfer struct {
	Token           Address
	From            Address
	To              Address
	Value           *big.Int
	TransactionHash TransactionHash
	LogIndex        uint
}

func IsERC20TransferLog(
	log ObservedLog,
) bool {
	if len(log.Topics) != 3 {
		return false
	}

	return common.HexToHash(
		log.Topics[0],
	) == erc20TransferTopic
}

func DecodeERC20Transfer(
	log ObservedLog,
	transactionHash TransactionHash,
) (ERC20Transfer, error) {
	if !IsERC20TransferLog(log) {
		return ERC20Transfer{},
			ErrNotERC20Transfer
	}

	if len(log.Data) != 32 {
		return ERC20Transfer{},
			ErrInvalidERC20TransferData
	}

	from :=
		addressFromTopic(
			log.Topics[1],
		)

	to :=
		addressFromTopic(
			log.Topics[2],
		)

	value :=
		new(big.Int).SetBytes(
			log.Data,
		)

	return ERC20Transfer{
		Token:           log.Address,
		From:            from,
		To:              to,
		Value:           value,
		TransactionHash: transactionHash,
		LogIndex:        log.Index,
	}, nil
}

func addressFromTopic(
	topic string,
) Address {
	hash :=
		common.HexToHash(topic)

	return Address(
		common.BytesToAddress(
			hash.Bytes()[12:],
		).Hex(),
	)
}
