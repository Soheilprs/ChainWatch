package main

import (
	"errors"
	"testing"
)

func TestTransferCursorEncodeDecode(
	t *testing.T,
) {
	expected :=
		TransferCursor{
			BlockNumber: 25871082,
			LogIndex:    48,
		}

	encoded, err :=
		EncodeTransferCursor(
			expected,
		)

	if err != nil {
		t.Fatalf(
			"expected encoding to succeed, got %v",
			err,
		)
	}

	if encoded == "" {
		t.Fatal(
			"expected encoded cursor",
		)
	}

	actual, err :=
		DecodeTransferCursor(
			encoded,
		)

	if err != nil {
		t.Fatalf(
			"expected decoding to succeed, got %v",
			err,
		)
	}

	if actual != expected {
		t.Fatalf(
			"expected %+v, got %+v",
			expected,
			actual,
		)
	}
}

func TestDecodeTransferCursorRejectsInvalidCursor(
	t *testing.T,
) {
	_, err :=
		DecodeTransferCursor(
			"definitely-not-valid",
		)

	if !errors.Is(
		err,
		ErrInvalidTransferCursor,
	) {
		t.Fatalf(
			"expected ErrInvalidTransferCursor, got %v",
			err,
		)
	}
}
