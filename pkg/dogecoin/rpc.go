package dogecoin

import (
	"fmt"
	"net/rpc"
	"net/rpc/jsonrpc"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// interface guard ensures L1CoreRPC implements giga.L1
var _ giga.L1 = L1CoreRPC{}

func NewDogecoinCoreRPC(config giga.Config) (L1CoreRPC, error) {
	// Connect to the dogecoin daemon
	addr := fmt.Sprintf("%s:%d", config.Dogecoind["testnet"].Rpcaddr, config.Dogecoind["testnet"].Rpcport)
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

func (d L1CoreRPC) MakeAddress() (giga.Address, error) {
	// res := map[string]struct{}{}
	res := ""
	err := d.client.Call("getrpcinfo", nil, &res)
	fmt.Println(res, err)
	return giga.Address{"foo", "bar"}, nil
}

func (d L1CoreRPC) Send(txn giga.Txn) error {
	return nil
}
