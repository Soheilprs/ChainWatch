package main

import "math/big"

type ObservedLog struct {
	Address         Address
	Topics          []string
	Data            []byte
	Index           uint
	TransactionHash TransactionHash
	BlockNumber     uint64
	Removed         bool
}

type ObservedReceipt struct {
	TransactionHash   TransactionHash
	Status            uint64
	GasUsed           uint64
	EffectiveGasPrice *big.Int
	ContractAddress   *Address
	Logs              []ObservedLog
}

func (r ObservedReceipt) Successful() bool {
	return r.Status == 1
}

func (r ObservedReceipt) LogCount() int {
	return len(r.Logs)
}

func (r ObservedReceipt) FeeWei() *big.Int {
	if r.EffectiveGasPrice == nil {
		return big.NewInt(0)
	}

	return new(big.Int).Mul(
		new(big.Int).SetUint64(r.GasUsed),
		r.EffectiveGasPrice,
	)
}
