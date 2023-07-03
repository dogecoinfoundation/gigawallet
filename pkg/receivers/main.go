package receivers

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
)

// Sets up standard receivers.
func SetUpReceivers(cond *conductor.Conductor, bus giga.MessageBus, conf giga.Config) {
	// Set up configured loggers
	SetupLoggers(cond, bus, conf)

	// InvoiceUpdater marks invoices paid when transactions have fully paid them.
	iup := NewInvoiceUpdater()
	cond.Service("InvoiceUpdater", iup)
	bus.Register(iup, giga.ACC_CHAIN_ACTIVITY)

	// BalanceTracker updates wallet balances after transactions are confirmed.
	btr := NewBalanceKeeper()
	cond.Service("BalanceTracker", btr)
	bus.Register(iup, giga.ACC_CHAIN_ACTIVITY)
}
