package services

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
)

func StartServices(cond *conductor.Conductor, bus giga.MessageBus, conf giga.Config, store giga.Store) {
	// BalanceKeeper updates stored balances and sends ACC_BALANCE_CHANGE events.
	keeper := NewBalanceKeeper(store, bus)
	cond.Service("NewBalanceKeeper", keeper)
}
