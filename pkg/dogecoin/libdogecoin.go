package dogecoin

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	"github.com/jaxlotl/go-libdogecoin"
)

// interface guard ensures L1Libdogecoin implements giga.L1
var _ giga.L1 = L1Libdogecoin{}

// NewL1Libdogecoin returns a giga.L1 implementor that uses libdogecoin
func NewL1Libdogecoin(config giga.Config) (L1Libdogecoin, error) {
	return L1Libdogecoin{}, nil
}

type L1Libdogecoin struct{}

func (d L1Libdogecoin) MakeAddress() (giga.Address, giga.Privkey, error) {
	libdogecoin.W_context_start()
	priv, pub := libdogecoin.W_generate_hd_master_pub_keypair(false)
	libdogecoin.W_context_stop()
	return giga.Address(pub), giga.Privkey(priv), nil
}

func (d L1Libdogecoin) MakeChildAddress(privkey giga.Privkey) (giga.Address, error) {
	libdogecoin.W_context_start()
	pub := libdogecoin.W_generate_derived_hd_pub_key(string(privkey))
	libdogecoin.W_context_stop()
	return giga.Address(pub), nil
}

func (d L1Libdogecoin) Send(txn giga.Txn) error {
	return nil
}
