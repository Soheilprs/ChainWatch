package main

type APITokenMetadata struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals uint8  `json:"decimals"`
}

type APITransfer struct {
	BlockNumber     uint64 `json:"blockNumber"`
	BlockHash       string `json:"blockHash"`
	TransactionHash string `json:"transactionHash"`
	LogIndex        uint   `json:"logIndex"`

	Token string `json:"token"`
	From  string `json:"from"`
	To    string `json:"to"`

	Value string `json:"value"`

	FormattedValue *string `json:"formattedValue,omitempty"`

	TokenMetadata *APITokenMetadata `json:"tokenMetadata,omitempty"`
}

type APIPagination struct {
	Limit      int     `json:"limit"`
	HasMore    bool    `json:"hasMore"`
	NextCursor *string `json:"nextCursor"`
}

type APITransferPage struct {
	Data       []APITransfer `json:"data"`
	Pagination APIPagination `json:"pagination"`
}

func apiTransferFromStored(
	transfer StoredERC20Transfer,
) APITransfer {
	value := "0"

	if transfer.Value != nil {
		value =
			transfer.Value.String()
	}

	return APITransfer{
		BlockNumber: transfer.BlockNumber,

		BlockHash: string(
			transfer.BlockHash,
		),

		TransactionHash: string(
			transfer.TransactionHash,
		),

		LogIndex: transfer.LogIndex,

		Token: string(
			transfer.Token,
		),

		From: string(
			transfer.From,
		),

		To: string(
			transfer.To,
		),

		Value: value,
	}
}

func enrichAPITransferWithMetadata(
	apiTransfer *APITransfer,
	transfer StoredERC20Transfer,
	metadata TokenMetadata,
) {
	formatted :=
		FormatTokenAmount(
			transfer.Value,
			metadata.Decimals,
		)

	apiTransfer.FormattedValue =
		&formatted

	apiTransfer.TokenMetadata =
		&APITokenMetadata{
			Name: metadata.Name,

			Symbol: metadata.Symbol,

			Decimals: metadata.Decimals,
		}
}
