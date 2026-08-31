package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/soheilprs/chainwatch/internal/domain"
)

var ErrInvalidTransferCursor = errors.New(
	"invalid transfer cursor",
)

type transferCursorPayload struct {
	BlockNumber uint64 `json:"blockNumber"`
	LogIndex    uint   `json:"logIndex"`
}

func EncodeTransferCursor(
	cursor domain.TransferCursor,
) (string, error) {
	payload :=
		transferCursorPayload{
			BlockNumber: cursor.BlockNumber,

			LogIndex: cursor.LogIndex,
		}

	data, err :=
		json.Marshal(payload)

	if err != nil {
		return "", fmt.Errorf(
			"failed to encode transfer cursor: %w",
			err,
		)
	}

	return base64.
			RawURLEncoding.
			EncodeToString(data),
		nil
}

func DecodeTransferCursor(
	value string,
) (domain.TransferCursor, error) {
	if value == "" {
		return domain.TransferCursor{},
			ErrInvalidTransferCursor
	}

	data, err :=
		base64.
			RawURLEncoding.
			DecodeString(value)

	if err != nil {
		return domain.TransferCursor{},
			ErrInvalidTransferCursor
	}

	var payload transferCursorPayload

	if err :=
		json.Unmarshal(
			data,
			&payload,
		); err != nil {

		return domain.TransferCursor{},
			ErrInvalidTransferCursor
	}

	return domain.TransferCursor{
		BlockNumber: payload.BlockNumber,

		LogIndex: payload.LogIndex,
	}, nil
}
