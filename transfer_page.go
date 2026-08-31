package main

type TransferPage struct {
	Transfers  []StoredERC20Transfer
	NextCursor *TransferCursor
}
