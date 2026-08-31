package api

import (
	"testing"

	"github.com/soheilprs/chainwatch/internal/domain"
)

var (
	benchmarkCursor  domain.TransferCursor
	benchmarkEncoded string
)

func BenchmarkTransferCursor(b *testing.B) {
	cursor := domain.TransferCursor{BlockNumber: 25_871_082, LogIndex: 48}
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
