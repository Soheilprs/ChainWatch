package main

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	benchmarkTransfer ERC20Transfer
	benchmarkAmount   string
	benchmarkCursor   TransferCursor
	benchmarkEncoded  string
)

func BenchmarkDecodeERC20Transfer(b *testing.B) {
	log := ObservedLog{
		Address: "0x3333333333333333333333333333333333333333",
		Topics: []string{
			erc20TransferTopic.Hex(),
			addressTopic("0x1111111111111111111111111111111111111111"),
			addressTopic("0x2222222222222222222222222222222222222222"),
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

func BenchmarkFormatTokenAmount(b *testing.B) {
	value, ok := new(big.Int).SetString("4661514008234536960", 10)
	if !ok {
		b.Fatal("create benchmark value")
	}

	b.ReportAllocs()
	for b.Loop() {
		benchmarkAmount = FormatTokenAmount(value, 18)
	}
}

func BenchmarkTransferCursor(b *testing.B) {
	cursor := TransferCursor{BlockNumber: 25_871_082, LogIndex: 48}
	encoded, err := EncodeTransferCursor(cursor)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("encode", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			value, err := EncodeTransferCursor(cursor)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkEncoded = value
		}
	})

	b.Run("decode", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			value, err := DecodeTransferCursor(encoded)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkCursor = value
		}
	})
}
