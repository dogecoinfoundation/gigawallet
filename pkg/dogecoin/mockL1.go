package core

import giga "github.com/dogecoinfoundation/gigawallet/pkg"

/* Returns a Mocked giga.DogecoinL1 implementor */
func NewL1Mock(config giga.Config) (DogecoinL1Mock, error) {

	return DogecoinL1Mock{}, nil

}

type DogecoinL1Mock struct {
}

func (d DogecoinL1Mock) MakeAddress() (giga.Address, error) {
	return giga.Address{"mockPrivKey", "mockPubKey"}, nil
}
