package domain

// BlockCheckpoint identifies the last durably indexed canonical block.
type BlockCheckpoint struct {
	Number uint64
	Hash   BlockHash
}
