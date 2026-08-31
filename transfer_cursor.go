package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrInvalidTransferCursor = errors.New(
	"invalid transfer cursor",
)

type transferCursorPayload struct {
	BlockNumber uint64 `json:"blockNumber"`
	LogIndex    uint   `json:"logIndex"`
}

func EncodeTransferCursor(
	cursor TransferCursor,
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
) (TransferCursor, error) {
	if value == "" {
		return TransferCursor{},
			ErrInvalidTransferCursor
	}

	data, err :=
		base64.
			RawURLEncoding.
			DecodeString(value)

	if err != nil {
		return TransferCursor{},
			ErrInvalidTransferCursor
	}

	var payload transferCursorPayload

	if err :=
		json.Unmarshal(
			data,
			&payload,
		); err != nil {

		return TransferCursor{},
			ErrInvalidTransferCursor
	}

	return TransferCursor{
		BlockNumber: payload.BlockNumber,

		LogIndex: payload.LogIndex,
	}, nil
}
