package main

import (
	"fmt"
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/broker"
	"github.com/dogecoinfoundation/gigawallet/pkg/core"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
	"github.com/tjstebbing/conductor"
)

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("usage: gigawallet <config-file> # e.g. devconf.toml\n")
		os.Exit(1)
	}
	conf := giga.LoadConfig(os.Args[1])
	if len(conf.Gigawallet.Dogecoind) < 1 {
		panic("bad config: missing gigawallet.dogecoind")
	}
	if len(conf.Dogecoind[conf.Gigawallet.Dogecoind].Host) < 1 {
		panic(fmt.Sprintf("bad config: missing dogecoind.%s.host", conf.Gigawallet.Dogecoind))
	}

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

	store, err := store.NewSQLite(conf.Store.DBFile)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	cf, err := broker.NewTxnConfirmer(conf)
	if err != nil {
		panic(err)
	}
	c.Service("Confirmer", cf)

	pb := broker.NewPaymentBroker(conf, store)
	c.Service("Payment Broker", pb)

	z, err := core.NewCoreReceiver(conf)
	if err != nil {
		panic(err)
	}
	z.Subscribe(cf.ReceiveFromNode)
	c.Service("ZMQ Listener", z)

	p, err := giga.NewWebAPI(conf, l1, store)
	if err != nil {
		panic(err)
	}
	c.Service("Payment API", p)

	<-c.Start()
}
