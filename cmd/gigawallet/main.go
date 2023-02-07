package main

import (
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/broker"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
	"github.com/dogecoinfoundation/gigawallet/pkg/core"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/dogecoinfoundation/gigawallet/pkg/messages"
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
)

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("usage: gigawallet <config-file> # e.g. devconf.toml\n")
		os.Exit(1)
	}
	conf := giga.LoadConfig(os.Args[1])

	rpc, err := dogecoin.NewL1Libdogecoin(conf, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(rpc.MakeAddress())

	c := conductor.NewConductor(
		conductor.HookSignals(),
		conductor.Noisy(),
	)

	// Start the MessageBus Service
	bus := giga.NewMessageBus()
	c.Service("MessageBus", bus)

	// Configure loggers
	messages.SetupLoggers(c, bus, conf)

	// Set up the L1 interface to Core
	l1_core, err := core.NewDogecoinCoreRPC(conf)
	if err != nil {
		panic(err)
	}
	l1, err := dogecoin.NewL1Libdogecoin(conf, l1_core)
	if err != nil {
		panic(err)
	}

	// Setup a Store, SQLite for now
	store, err := store.NewSQLite(conf.Store.DBFile)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	// Start the TxnConfirmer service
	cf, err := broker.NewTxnConfirmer(conf)
	if err != nil {
		panic(err)
	}
	c.Service("Confirmer", cf)

	// Start the PaymentBroker service
	pb := broker.NewPaymentBroker(conf, store)
	c.Service("Payment Broker", pb)

	// Start the Core listener service (ZMQ)
	z, err := core.NewCoreReceiver(bus, conf)
	if err != nil {
		panic(err)
	}
	z.Subscribe(cf.ReceiveFromNode)
	c.Service("ZMQ Listener", z)
	// Start the Payment API
	p, err := giga.NewWebAPI(conf, l1, store)
	if err != nil {
		panic(err)
	}
	c.Service("Payment API", p)

	<-c.Start()
}
