package main

import (
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/core"
)

func main() {
	conf := giga.LoadConfig(os.Args[1])
	rpc, err := core.NewDogecoinCoreRPC(conf)
	if err != nil {
		panic(err)
	}
	rpc.MakeAddress()
}
