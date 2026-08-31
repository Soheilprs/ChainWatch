package metadata

import (
	"math/big"
	"testing"
)

var benchmarkAmount string

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
