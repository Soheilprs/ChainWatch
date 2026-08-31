package main

import (
	"math/big"
	"testing"
)

func TestFormatTokenAmountUSDC(
	t *testing.T,
) {
	value :=
		big.NewInt(
			1881330000,
		)

	result :=
		FormatTokenAmount(
			value,
			6,
		)

	if result != "1881.33" {
		t.Fatalf(
			"expected 1881.33, got %s",
			result,
		)
	}
}

func TestFormatTokenAmountWholeNumber(
	t *testing.T,
) {
	value :=
		big.NewInt(
			30000000,
		)

	result :=
		FormatTokenAmount(
			value,
			6,
		)

	if result != "30" {
		t.Fatalf(
			"expected 30, got %s",
			result,
		)
	}
}

func TestFormatTokenAmountSmallValue(
	t *testing.T,
) {
	value :=
		big.NewInt(
			100,
		)

	result :=
		FormatTokenAmount(
			value,
			6,
		)

	if result != "0.0001" {
		t.Fatalf(
			"expected 0.0001, got %s",
			result,
		)
	}
}

func TestFormatTokenAmountEighteenDecimals(
	t *testing.T,
) {
	value, ok :=
		new(big.Int).SetString(
			"4661514008234536960",
			10,
		)

	if !ok {
		t.Fatal(
			"failed to create big integer",
		)
	}

	result :=
		FormatTokenAmount(
			value,
			18,
		)

	if result !=
		"4.66151400823453696" {

		t.Fatalf(
			"unexpected formatted value %s",
			result,
		)
	}
}

func TestFormatTokenAmountZeroDecimals(
	t *testing.T,
) {
	result :=
		FormatTokenAmount(
			big.NewInt(123),
			0,
		)

	if result != "123" {
		t.Fatalf(
			"expected 123, got %s",
			result,
		)
	}
}
