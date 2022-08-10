package dogecoin

import giga "github.com/dogecoinfoundation/gigawallet/pkg"

var _ giga.DogecoinL1 = DogecoinL1Mock{}

/* Returns a Mocked giga.DogecoinL1 implementor */
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
