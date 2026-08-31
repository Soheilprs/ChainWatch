package main

type BlockTransferIndex struct {
	BlockNumber uint64
	BlockHash   BlockHash
	ParentHash  BlockHash
	Transfers   []ERC20Transfer
}

func (index BlockTransferIndex) TransferCount() int {
	return len(index.Transfers)
}

func (index BlockTransferIndex) HasTransfers() bool {
	return len(index.Transfers) > 0
}
