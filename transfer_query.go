package main

type TransferCursor struct {
	BlockNumber uint64
	LogIndex    uint
}

type TransferQuery struct {
	BlockNumber *uint64
	Token       *Address
	Address     *Address
	Limit       int
	Cursor      *TransferCursor
}
