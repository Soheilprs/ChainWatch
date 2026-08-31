package main

import (
	"math/big"
	"strings"
)

func FormatTokenAmount(
	value *big.Int,
	decimals uint8,
) string {
	if value == nil {
		return "0"
	}

	if decimals == 0 {
		return value.String()
	}

	negative :=
		value.Sign() < 0

	absolute :=
		new(big.Int).Abs(
			new(big.Int).Set(
				value,
			),
		)

	digits :=
		absolute.String()

	decimalCount :=
		int(decimals)

	if len(digits) <= decimalCount {
		digits =
			strings.Repeat(
				"0",
				decimalCount-len(digits)+1,
			) +
				digits
	}

	split :=
		len(digits) -
			decimalCount

	whole :=
		digits[:split]

	fraction :=
		digits[split:]

	fraction =
		strings.TrimRight(
			fraction,
			"0",
		)

	result := whole

	if fraction != "" {
		result +=
			"." +
				fraction
	}

	if negative {
		result =
			"-" +
				result
	}

	return result
}
