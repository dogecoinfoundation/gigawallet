package receivers

import (
	"context"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

/** The Balance Keeper.
 *
 *  Each time an invoice is confirmed-paid (or any other UTXO is confirmed)
 *  the BalanceKeeper updates the balance on the account.
 *
 *  If a confirmed UTXO is later rolled back, the BalanceKeeper must also
 *  subtract its value from the account balance.
 *
 *  Pending funds: non-confirmed UTXOs add to the `Incoming` funds balance.
 *  Pending payments: non-confirmed Payment transactions add to the `Outgoing` funds.
 *
 *  The easiest way to implement this is to SUM() across all UTXOs.
 *
 *  How do we know an account needs updating?
 *   - when UTXOs are added or marked spent;
 *   - (invoice.confirmations or config.confirmations) after UTXO added (invoices)
 *   - (payment.confirmations or config.confirmations) after UTXO spent (payments)
 */
type BalanceKeeper struct {
	// BalanceKeeper receives giga.Message via Rec
	Rec chan giga.Message
}

func NewBalanceKeeper() BalanceKeeper {
	// create an BalanceTracker
	btr := BalanceKeeper{
		make(chan giga.Message, 100),
	}
	return btr
}

// Implements giga.MessageSubscriber
func (b BalanceKeeper) GetChan() chan giga.Message {
	return b.Rec
}

// Implements conductor.Service
func (b BalanceKeeper) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		started <- true
		for {
			select {
			// handle stopping the service
			case <-stop:
				close(b.Rec)
				close(stopped)
				return
			case msg := <-b.Rec:
				msg = msg
				// l.Log.Printf("%s:%s (%s): %s\n",
				// 	msg.EventType.Type(),
				// 	msg.EventType,
				// 	msg.ID,
				// 	msg.Message)
			default:
				b.processAccount()
			}
		}
	}()
	return nil
}

func (l BalanceKeeper) processAccount() {
}
