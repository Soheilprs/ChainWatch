package main

type BlockchainClient interface {
	GetLatestBlock() (Block, error)
}

type MockBlockchainClient struct {
	Block Block
	Err   error
}

var _ BlockchainClient = MockBlockchainClient{}

func (m MockBlockchainClient) GetLatestBlock() (Block, error) {
	if m.Err != nil {
		return Block{}, m.Err
	}

	return m.Block, nil
}
