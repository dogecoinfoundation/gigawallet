package dogecoin

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	"github.com/dogeorg/go-libdogecoin"
)

// interface guard ensures L1Libdogecoin implements giga.L1
var _ giga.L1 = L1Libdogecoin{}

// NewL1Libdogecoin returns a giga.L1 implementor that uses libdogecoin
func NewL1Libdogecoin(config giga.Config) (L1Libdogecoin, error) {
	return L1Libdogecoin{}, nil
}

type L1Libdogecoin struct{}

func (l L1Libdogecoin) MakeAddress() (giga.Address, giga.Privkey, error) {
	libdogecoin.W_context_start()
	priv, pub := libdogecoin.W_generate_hd_master_pub_keypair(false)
	libdogecoin.W_context_stop()
	return giga.Address(pub), giga.Privkey(priv), nil
}

func (l L1Libdogecoin) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	libdogecoin.W_context_start()
	// this API is a bit odd: it returns the "extended public key"
	// which you can think of as a coordinate in the HD Wallet key-space.
	hd_node := libdogecoin.W_get_derived_hd_address(string(privkey), 0, isInternal, addressIndex, false)
	// derive the dogecoin address (hash) from the extended public-key
	pub := libdogecoin.W_generate_derived_hd_pub_key(hd_node)
	libdogecoin.W_context_stop()
	return giga.Address(pub), nil
}

func (l L1Libdogecoin) Send(txn giga.Txn) error {
	return nil
}
