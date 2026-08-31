package domain

import "math/big"

type ObservedTransaction struct {
	Hash     TransactionHash
	From     Address
	To       *Address
	ValueWei *big.Int
	Nonce    uint64
	GasLimit uint64
	Type     uint8
}

type ObservedBlock struct {
	Number       uint64
	Hash         BlockHash
	ParentHash   BlockHash
	Timestamp    uint64
	Transactions []ObservedTransaction
}

func (block ObservedBlock) TransactionCount() int {
	return len(block.Transactions)
}
