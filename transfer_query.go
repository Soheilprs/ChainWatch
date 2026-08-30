package main

type TransferQuery struct {
	BlockNumber *uint64
	Token       *Address
	Address     *Address
	Limit       int
}
