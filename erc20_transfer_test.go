package main

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func addressTopic(
	address string,
) string {
	return common.BytesToHash(
		common.HexToAddress(
			address,
		).Bytes(),
	).Hex()
}

func TestDecodeERC20Transfer(
	t *testing.T,
) {
	from :=
		"0x1111111111111111111111111111111111111111"

	to :=
		"0x2222222222222222222222222222222222222222"

	token := Address(
		"0x3333333333333333333333333333333333333333",
	)

	log := ObservedLog{
		Address: token,
		Topics: []string{
			erc20TransferTopic.Hex(),
			addressTopic(from),
			addressTopic(to),
		},
		Data: common.LeftPadBytes(
			big.NewInt(1000).Bytes(),
			32,
		),
	}

	transfer, err := DecodeERC20Transfer(
		log,
		"0xtx123",
	)

	if err != nil {
		t.Fatalf(
			"expected transfer to decode, got %v",
			err,
		)
	}

	expectedFrom := Address(
		common.HexToAddress(from).Hex(),
	)

	if transfer.From != expectedFrom {
		t.Errorf(
			"expected from %s, got %s",
			expectedFrom,
			transfer.From,
		)
	}

	expectedTo := Address(
		common.HexToAddress(to).Hex(),
	)

	if transfer.To != expectedTo {
		t.Errorf(
			"expected to %s, got %s",
			expectedTo,
			transfer.To,
		)
	}

	if transfer.Token != token {
		t.Errorf(
			"expected token %s, got %s",
			token,
			transfer.Token,
		)
	}

	if transfer.Value.Cmp(
		big.NewInt(1000),
	) != 0 {
		t.Errorf(
			"expected value 1000, got %s",
			transfer.Value.String(),
		)
	}

	if transfer.TransactionHash != "0xtx123" {
		t.Errorf(
			"expected transaction hash 0xtx123, got %s",
			transfer.TransactionHash,
		)
	}
}

func TestIsERC20TransferLogFalseForDifferentEvent(
	t *testing.T,
) {
	owner := "0x1111111111111111111111111111111111111111"
	spender := "0x2222222222222222222222222222222222222222"
	token := Address(
		"0x3333333333333333333333333333333333333333",
	)

	var erc20ApprovalTopic = crypto.Keccak256Hash(
		[]byte("Approval(address,address,uint256)"),
	)

	log := ObservedLog{
		Address: token,
		Topics: []string{
			erc20ApprovalTopic.Hex(),
			addressTopic(owner),
			addressTopic(spender),
		},
		Data: common.LeftPadBytes(
			big.NewInt(1000).Bytes(),
			32,
		),
	}

	if IsERC20TransferLog(log) {
		t.Fatal("expected log not to be identified as an ERC20 transfer")
	}
}

func TestDecodeERC20TransferInvalidData(t *testing.T) {
	from :=
		"0x1111111111111111111111111111111111111111"

	to :=
		"0x2222222222222222222222222222222222222222"

	token := Address(
		"0x3333333333333333333333333333333333333333",
	)

	log := ObservedLog{
		Address: token,
		Topics: []string{
			erc20TransferTopic.Hex(),
			addressTopic(from),
			addressTopic(to),
		},
		Data: []byte{1, 2, 3},
	}

	_, err := DecodeERC20Transfer(
		log,
		"0xtx123",
	)

	if !errors.Is(
		err,
		ErrInvalidERC20TransferData,
	) {
		t.Fatalf("expected invalid ERC20 transfer data, got: %v", err)
	}

}
