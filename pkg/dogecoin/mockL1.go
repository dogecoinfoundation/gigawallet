package dogecoin

import giga "github.com/dogecoinfoundation/gigawallet/pkg"

// interface guard ensures DogecoinL1Mock implements giga.DogecoinL1
var _ giga.DogecoinL1 = DogecoinL1Mock{}

// NewL1Mock returns a mocked giga.DogecoinL1 implementor
func NewL1Mock(config giga.Config) (DogecoinL1Mock, error) {
	return DogecoinL1Mock{}, nil
}

type DogecoinL1Mock struct {
}

func (d DogecoinL1Mock) MakeAddress() (giga.Address, error) {
	return giga.Address{"mockPrivKey", "mockPubKey"}, nil
}

func (d DogecoinL1Mock) Send(txn giga.Txn) error {
	return nil
}
