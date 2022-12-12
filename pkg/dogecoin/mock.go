package dogecoin

import (
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// interface guard ensures L1Mock implements giga.L1
var _ giga.L1 = L1Mock{}

// NewL1Mock returns a mocked giga.L1 implementor
func NewL1Mock(config giga.Config) (L1Mock, error) {
	return L1Mock{}, nil
}

type L1Mock struct{}

func (l L1Mock) MakeAddress() (giga.Address, giga.Privkey, error) {
	return "mockAddress", "mockPrivkey", nil
}

func (l L1Mock) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	return "mockChildAddress", nil
}

func (l L1Mock) MakeTransaction(amount giga.Koinu, UTXOs []giga.UTXO, payTo giga.Address, fee giga.Koinu, change giga.Address) (giga.Txn, error) {
	return giga.Txn{}, fmt.Errorf("not implemented")
}

func (l L1Mock) Send(txn giga.Txn) error {
	return nil
}
