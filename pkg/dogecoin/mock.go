package dogecoin

import giga "github.com/dogecoinfoundation/gigawallet/pkg"

// interface guard ensures L1Mock implements giga.L1
var _ giga.L1 = L1Mock{}

// NewL1Mock returns a mocked giga.L1 implementor
func NewL1Mock(config giga.Config) (L1Mock, error) {
	return L1Mock{}, nil
}

type L1Mock struct{}

func (d L1Mock) MakeAddress() (giga.Address, giga.Privkey, error) {
	return "mockAddress", "mockPrivkey", nil
}

func (d L1Mock) Send(txn giga.Txn) error {
	return nil
}
