package main

import "math/big"

type StoredERC20Transfer struct {
	BlockNumber     uint64
	BlockHash       BlockHash
	TransactionHash TransactionHash
	LogIndex        uint

	Token Address
	From  Address
	To    Address

	Value *big.Int
}
