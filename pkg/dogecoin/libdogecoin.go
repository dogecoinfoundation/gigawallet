package dogecoin

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	"github.com/jaxlotl/go-libdogecoin"
)

// interface guard ensures DogecoinL1Libdogecoin implements giga.DogecoinL1
var _ giga.DogecoinL1 = DogecoinL1Libdogecoin{}

// NewL1Libdogecoin returns a giga.DogecoinL1 implementor that uses libdogecoin
func NewL1Libdogecoin(config giga.Config) (DogecoinL1Libdogecoin, error) {
	return DogecoinL1Libdogecoin{}, nil
}

type DogecoinL1Libdogecoin struct {
}

func (d DogecoinL1Libdogecoin) Send(txn giga.Txn) error {
	return nil
}

func (d DogecoinL1Libdogecoin) MakeAddress() (giga.Address, error) {
	libdogecoin.W_context_start()
	priv, pub := libdogecoin.W_generate_hd_master_pub_keypair(false)
	libdogecoin.W_context_stop()
	return giga.Address{priv, pub}, nil
}
