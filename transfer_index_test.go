package main

import "testing"

func TestBlockTransferIndexTransferCount(
	t *testing.T,
) {
	index := BlockTransferIndex{
		Transfers: []ERC20Transfer{
			{},
			{},
			{},
		},
	}

	if index.TransferCount() != 3 {
		t.Fatalf(
			"expected 3 transfers, got %d",
			index.TransferCount(),
		)
	}
}

func TestBlockTransferIndexHasTransfersFalse(
	t *testing.T,
) {
	index := BlockTransferIndex{}

	if index.HasTransfers() {
		t.Fatal(
			"expected index to have no transfers",
		)
	}
}
