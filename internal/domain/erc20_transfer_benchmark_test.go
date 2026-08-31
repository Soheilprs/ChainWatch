package domain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var benchmarkTransfer ERC20Transfer

func BenchmarkDecodeERC20Transfer(b *testing.B) {
	log := ObservedLog{
		Address: "0x3333333333333333333333333333333333333333",
		Topics: []string{
			erc20TransferTopic.Hex(),
			benchmarkAddressTopic("0x1111111111111111111111111111111111111111"),
			benchmarkAddressTopic("0x2222222222222222222222222222222222222222"),
		},
		Data: common.LeftPadBytes(big.NewInt(1_881_330_000).Bytes(), 32),
	}

	b.ReportAllocs()
	for b.Loop() {
		transfer, err := DecodeERC20Transfer(log, "0xtransaction")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkTransfer = transfer
	}
}

func benchmarkAddressTopic(address string) string {
	return common.BytesToHash(common.HexToAddress(address).Bytes()).Hex()
}
