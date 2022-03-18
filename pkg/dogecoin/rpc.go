package core

import (
	"fmt"
	"net/rpc"
	"net/rpc/jsonrpc"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

func NewDogecoinCoreRPC(config giga.Config) (DogecoinCoreRPC, error) {
	// Connect to the dogecoin daemon
	addr := fmt.Sprintf("%s:%d", config.Dogecoind["testnet"].Rpcaddr, config.Dogecoind["testnet"].Rpcport)
	fmt.Println("Dialing:", addr)
	c, err := jsonrpc.Dial("tcp", addr)
	if err != nil {
		return DogecoinCoreRPC{}, err
	}
	fmt.Println("Dialed")

	return DogecoinCoreRPC{c}, nil
}

type DogecoinCoreRPC struct {
	client *rpc.Client
}

func (d DogecoinCoreRPC) MakeAddress() (giga.Address, error) {
	// res := map[string]struct{}{}
	res := ""
	err := d.client.Call("getrpcinfo", nil, &res)
	fmt.Println(res, err)
	return giga.Address{"foo", "bar"}, nil
}
