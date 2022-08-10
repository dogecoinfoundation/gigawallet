package main

import (
	"fmt"
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
	c.Service("Payment API", giga.PaymentAPIService{})
	<-c.Start()
}
