package main

import (
	"fmt"
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/tjstebbing/conductor"
)

func main() {
	conf := giga.LoadConfig(os.Args[1])
	rpc, err := dogecoin.NewL1Libdogecoin(conf)
	if err != nil {
		panic(err)
	}
	fmt.Println(rpc.MakeAddress())
	c := conductor.New(
		conductor.HookSignals(),
		conductor.Noisy(),
	)
	l1, err := dogecoin.NewL1Libdogecoin(conf)
	if err != nil {
		panic(err)
	}
	p, err := giga.NewWebAPI(conf, l1, store.NewMock())
	if err != nil {
		panic(err)
	}
	c.Service("Payment API", p)
	<-c.Start()
}
