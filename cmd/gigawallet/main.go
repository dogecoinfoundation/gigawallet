package main

import (
	"fmt"
	"os"

	"github.com/dogecoinfoundation/gigawallet/pkg/store"

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

	st, err := store.NewSQLite("gigawallet.db")
	if err != nil {
		panic(err)
	}
	defer st.Close()

	p, err := giga.NewWebAPI(conf, l1, st)
	if err != nil {
		panic(err)
	}
	c.Service("Payment API", p)
	<-c.Start()
}
