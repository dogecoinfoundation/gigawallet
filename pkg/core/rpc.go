package core

import (
	"fmt"
	"net/rpc"
	"net/rpc/jsonrpc"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// interface guard ensures L1CoreRPC implements giga.L1
var _ giga.L1 = L1CoreRPC{}

// NewDogecoinCoreRPC returns a giga.L1 implementor that uses dogecoin-core's RPC
func NewDogecoinCoreRPC(config giga.Config) (L1CoreRPC, error) {
	// Connect to the dogecoin daemon
	addr := fmt.Sprintf("%s:%d", config.Dogecoind[config.Gigawallet.Dogecoind].Host, config.Dogecoind[config.Gigawallet.Dogecoind].RPCPort)
	fmt.Println("Dialing:", addr)
	c, err := jsonrpc.Dial("tcp", addr)
	if err != nil {
		return L1CoreRPC{}, err
	}
	fmt.Println("Dialed")

	return L1CoreRPC{c}, nil
}

type L1CoreRPC struct {
	client *rpc.Client
}

func (l L1CoreRPC) MakeAddress() (giga.Address, giga.Privkey, error) {
	// res := map[string]struct{}{}
	res := ""
	err := l.client.Call("getrpcinfo", nil, &res)
	fmt.Println(res, err)
	return "foo", "bar", nil
}

func (l L1CoreRPC) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	return "foo", nil
}

func (l L1CoreRPC) Send(txn giga.Txn) error {
	return nil
}
