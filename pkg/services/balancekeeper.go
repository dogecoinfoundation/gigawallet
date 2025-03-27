package services

import (
	"context"
	"fmt"
	"log"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

const (
	RETRY_DELAY        = 5 * time.Second // for Database errors.
	CONFLICT_DELAY     = 1 * time.Second // for Database conflicts (concurrent transactions)
	DELAY_BETWEEN_RUNS = 1 * time.Second // time between batch queries
	ACCOUNT_BATCH_SIZE = 1               // number of Accounts to process at once
	INVOICE_PAGE_SIZE  = 10              // number of Invoices to fetch per page
	SERVICE_KEY        = "BalanceKeeper" // service name, stored in the database
)

type BalanceKeeper struct {
	store giga.Store
	bus   giga.MessageBus
	stop  chan context.Context  // service stop
	tx    giga.StoreTransaction // non-nil during a transaction (for shutdown)
}

func NewBalanceKeeper(store giga.Store, bus giga.MessageBus) BalanceKeeper {
	keeper := BalanceKeeper{
		store: store,
		bus:   bus,
		stop:  nil,
	}
	return keeper
}

// Implements conductor.Service
func (b BalanceKeeper) Run(started, stopped chan bool, stop chan context.Context) error {
	b.stop = stop
	go func() {
		// Recover from panic used to stop or restart the service.
		defer func() {
			if r := recover(); r != nil {
				log.Println("BalanceKeeper: panic received:", r)
				stopped <- true
			}
			if b.tx != nil {
				// shutdown during a transaction.
				b.tx.Rollback()
				b.tx = nil
			}
		}()
		started <- true
		cursor := b.fetchServiceCursor(SERVICE_KEY)
		for {
			select {
			case <-stop:
				close(stopped)
				return
			default:
				newCursor, err := b.runBatch(cursor)
				if err != nil {
					b.sleepForRetry(err, 0)
					continue // retry.
				} else {
					b.sleepForRetry(nil, DELAY_BETWEEN_RUNS)
				}
				cursor = newCursor // advance the cursor.
			}
		}
	}()
	return nil
}

func (b BalanceKeeper) runBatch(cursor int64) (int64, error) {
	tx := b.beginStoreTxn()
	ids, newCursor, err := tx.ListAccountsModifiedSince(cursor, ACCOUNT_BATCH_SIZE)
	if err != nil {
		tx.Rollback()
		log.Println("BalanceKeeper: ListAccountsModifiedSince:", err)
		return cursor, err
	}
	for n, id := range ids {
		err = b.updateAccountBalance(tx, giga.Address(id), cursor, n)
		if err != nil {
			tx.Rollback()
			return cursor, err // already logged.
		}
	}
	if newCursor > cursor {
		err = tx.SetServiceCursor(SERVICE_KEY, newCursor)
		if err != nil {
			tx.Rollback()
			log.Println("BalanceKeeper: SetServiceCursor:", err)
			return cursor, err
		}
	}
	err = tx.Commit()
	b.tx = nil // for shutdown.
	if err != nil {
		log.Println("BalanceKeeper: Commit:", err)
		return cursor, err
	}
	return newCursor, nil
}

func (b BalanceKeeper) updateAccountBalance(tx giga.StoreTransaction, id giga.Address, cursor int64, n int) error {
	log.Printf("BalanceKeeper: checking account balance: %s\n", id)
	acc, err := tx.GetAccountByID(id)
	if err != nil {
		log.Printf("BalanceKeeper: GetAccountByID '%s': %v\n", id, err)
		return err
	}
	// Balance.
	bal, err := tx.CalculateBalance(giga.Address(id))
	if err != nil {
		log.Printf("BalanceKeeper: CalculateBalance '%s': %v\n", id, err)
		return err
	}
	log.Printf("BalanceKeeper: account balance '%s': in %v bal %v out %v\n", id, bal.IncomingBalance, bal.CurrentBalance, bal.OutgoingBalance)
	if !(bal.CurrentBalance.Equals(acc.CurrentBalance) &&
		bal.IncomingBalance.Equals(acc.IncomingBalance) &&
		bal.OutgoingBalance.Equals(acc.OutgoingBalance)) {
		// update the stored account balance.
		err = tx.UpdateAccountBalance(acc.Address, bal)
		if err != nil {
			log.Printf("BalanceKeeper: UpdateAccountBalance '%s': %v\n", id, err)
			return err
		}
		msg := giga.AccBalanceChangeEvent{
			AccountID:       acc.Address,
			ForeignID:       acc.ForeignID,
			CurrentBalance:  bal.CurrentBalance,
			IncomingBalance: bal.IncomingBalance,
			OutgoingBalance: bal.OutgoingBalance,
		}
		unique_id := fmt.Sprintf("ABC-%d-%d", cursor, n)
		err = b.bus.Send(giga.ACC_BALANCE_CHANGE, msg, unique_id)
		if err != nil {
			log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
			return err
		}
		log.Printf("BalanceKeeper: updated account balance: %s\n", id)
	}
	err = b.sendInvoiceEvents(tx, &acc, id, cursor, n)
	if err != nil {
		return err
	}
	return b.sendPaymentEvents(tx, &acc, id, cursor, n)
}

// Invoices.
func (b BalanceKeeper) sendInvoiceEvents(tx giga.StoreTransaction, acc *giga.Account, id giga.Address, cursor int64, n int) error {
	log.Printf("BalanceKeeper: checking invoices: %s\n", id)
	inv_c := 0
	num_inv := 0
	for cont := true; cont; cont = inv_c > 0 {
		// Fetch a batch of invoices.
		invoices, new_inv_c, err := tx.ListInvoices(id, inv_c, ACCOUNT_BATCH_SIZE)
		if err != nil {
			log.Printf("BalanceKeeper: ListInvoices '%s': %v\n", id, err)
			return err
		}
		// Check each invoice to see if it's paid and we haven't sent an event yet,
		// or other changes in amounts paid.
		for n, inv := range invoices {

			// Send INV_BALANCE_CHANGED if IncomingAmount or PaidAmount have changed.
			incomingChanged := !inv.IncomingAmount.Equals(inv.LastIncomingAmount)
			paidChanged := !inv.PaidAmount.Equals(inv.LastPaidAmount)
			if incomingChanged || paidChanged {
				event := giga.INV_BALANCE_CHANGED
				msg := giga.InvPaymentEvent{
					InvoiceID:      inv.ID,
					AccountID:      acc.Address,
					ForeignID:      acc.ForeignID,
					InvoiceTotal:   inv.Total,
					TotalIncoming:  inv.IncomingAmount,
					TotalConfirmed: inv.PaidAmount,
				}
				unique_id := fmt.Sprintf("IBC-%d-%d", cursor, n)
				err = b.bus.Send(event, msg, unique_id)
				if err != nil {
					log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
					return err
				}
				b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %s in %s\n", event, inv.ID, id))
			}

			// need a way to detect:
			// new unconfirmed payment: IncomingAmount > LastIncomingAmount
			// invoice fully paid (in Store): PaidHeight set
			// rollbacks: PaidHeight unset & PaidEvent set; PaidTotal < LastPaidTotal; Incoming < LastIncoming
			// NB. IncomingAmount doesn't reduce when payments are confirmed (unlike
			// IncomingBalance on Account) because it simplifies this logic:

			// Detect and send Payment Detected events (not yet confirmed)
			if inv.IncomingAmount.GreaterThan(inv.LastIncomingAmount) {
				// incoming amount has increased.
				// need to avoid reporting PART/TOTAL again after we report TOTAL.
				if inv.LastIncomingAmount.LessThan(inv.Total) {
					event := giga.INV_PART_PAYMENT_DETECTED
					if inv.IncomingAmount.GreaterThanOrEqual(inv.Total) {
						event = giga.INV_TOTAL_PAYMENT_DETECTED
					}
					// notify BUS listeners.
					msg := giga.InvPaymentEvent{
						InvoiceID:      inv.ID,
						AccountID:      acc.Address,
						ForeignID:      acc.ForeignID,
						InvoiceTotal:   inv.Total,
						TotalIncoming:  inv.IncomingAmount,
						TotalConfirmed: inv.PaidAmount,
					}
					unique_id := fmt.Sprintf("IPD-%d-%d", cursor, num_inv+n)
					err = b.bus.Send(event, msg, unique_id)
					if err != nil {
						log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
						return err
					}
					b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %s in %s\n", event, inv.ID, id))
				}
				// detect and report over-payments.
				if inv.IncomingAmount.GreaterThan(inv.Total) {
					// notify BUS listeners.
					msg := giga.InvPaymentEvent{
						InvoiceID:      inv.ID,
						AccountID:      acc.Address,
						ForeignID:      acc.ForeignID,
						InvoiceTotal:   inv.Total,
						TotalIncoming:  inv.IncomingAmount,
						TotalConfirmed: inv.PaidAmount,
					}
					event := giga.INV_OVER_PAYMENT_DETECTED
					unique_id := fmt.Sprintf("IPO-%d-%d", cursor, num_inv+n)
					err = b.bus.Send(event, msg, unique_id)
					if err != nil {
						log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
						return err
					}
					b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %s in %s\n", event, inv.ID, id))
				}
			}

			// Detect and send Payment Confirmed / Unconfirmed (fork) events
			if inv.PaidHeight != 0 && inv.PaidEvent.IsZero() {
				// invoice is fully paid and confirmed.
				// notify BUS listeners.
				msg := giga.InvPaymentEvent{
					InvoiceID:      inv.ID,
					AccountID:      acc.Address,
					ForeignID:      acc.ForeignID,
					InvoiceTotal:   inv.Total,
					TotalIncoming:  inv.IncomingAmount,
					TotalConfirmed: inv.PaidAmount,
				}
				event := giga.INV_TOTAL_PAYMENT_CONFIRMED
				unique_id := fmt.Sprintf("IPC-%d-%d", cursor, num_inv+n)
				err = b.bus.Send(event, msg, unique_id)
				if err != nil {
					log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
					return err
				}
				err = tx.MarkInvoiceEventSent(inv.ID, event)
				if err != nil {
					log.Printf("BalanceKeeper: MarkInvoiceEventSent '%s': %v\n", inv.ID, err)
					return err
				}
				b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %s in %s\n", event, inv.ID, id))
			} else if inv.PaidHeight == 0 && !inv.PaidEvent.IsZero() {
				// rollback detected.
				msg := giga.InvPaymentEvent{
					InvoiceID:      inv.ID,
					AccountID:      acc.Address,
					ForeignID:      acc.ForeignID,
					InvoiceTotal:   inv.Total,
					TotalIncoming:  inv.IncomingAmount,
					TotalConfirmed: inv.PaidAmount,
				}
				event := giga.INV_PAYMENT_UNCONFIRMED
				unique_id := fmt.Sprintf("IPU-%d-%d", cursor, num_inv+n)
				err = b.bus.Send(event, msg, unique_id)
				if err != nil {
					log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
					return err
				}
				err = tx.MarkInvoiceEventSent(inv.ID, event)
				if err != nil {
					log.Printf("BalanceKeeper: MarkInvoiceEventSent '%s': %v\n", inv.ID, err)
					return err
				}
				b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %s in %s\n", event, inv.ID, id))
			}

			// Detect and send Overpayment Confirmed events
			if inv.PaidAmount.GreaterThan(inv.LastPaidAmount) && inv.PaidAmount.GreaterThan(inv.Total) {
				// an overpayment was confirmed.
				msg := giga.InvOverpaymentEvent{
					InvoiceID:            inv.ID,
					AccountID:            acc.Address,
					ForeignID:            acc.ForeignID,
					InvoiceTotal:         inv.Total,
					TotalIncoming:        inv.IncomingAmount,
					TotalConfirmed:       inv.PaidAmount,
					OverpaymentIncoming:  inv.IncomingAmount,
					OverpaymentConfirmed: inv.PaidAmount,
				}
				event := giga.INV_OVER_PAYMENT_CONFIRMED
				unique_id := fmt.Sprintf("IOC-%d-%d", cursor, num_inv+n)
				err = b.bus.Send(event, msg, unique_id)
				if err != nil {
					log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
					return err
				}
				b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %s in %s\n", event, inv.ID, id))
			}

			// Update the DB to avoid repeated events.
			if incomingChanged {
				// This sets LastIncomingAmount to IncomingAmount (event name just selects update mode)
				err = tx.MarkInvoiceEventSent(inv.ID, giga.INV_PART_PAYMENT_DETECTED)
				if err != nil {
					log.Printf("BalanceKeeper: MarkInvoiceEventSent: '%s': %v\n", inv.ID, err)
					return err
				}
			}
			if paidChanged {
				// This sets LastPaidAmount to PaidAmount (event name just selects update mode)
				err = tx.MarkInvoiceEventSent(inv.ID, giga.INV_OVER_PAYMENT_CONFIRMED)
				if err != nil {
					log.Printf("BalanceKeeper: MarkInvoiceEventSent: '%s': %v\n", inv.ID, err)
					return err
				}
			}
		}
		num_inv += len(invoices)
		inv_c = new_inv_c
	}
	log.Printf("BalanceKeeper: checked %d invoices: %s\n", num_inv, id)
	return nil
}

// Payments.
func (b BalanceKeeper) sendPaymentEvents(tx giga.StoreTransaction, acc *giga.Account, id giga.Address, cursor int64, n int) error {
	log.Printf("BalanceKeeper: checking payments: %s\n", id)
	var pay_c int64 = 0
	num_pay := 0
	for cont := true; cont; cont = pay_c > 0 {
		// Fetch a batch of payments.
		payments, new_pay_c, err := tx.ListPayments(id, pay_c, ACCOUNT_BATCH_SIZE)
		if err != nil {
			log.Printf("BalanceKeeper: ListPayments '%s': %v\n", id, err)
			return err
		}
		// Check each payment to see if it's confirmed.
		for n, pay := range payments {
			// on paid_height <> 0 && no onchain_event => PAYMENT_ON_CHAIN
			// on confirmed_height <> 0 && no confirmed_event => PAYMENT_CONFIRMED
			// on confirmed_height = 0 && has confirmed_event => PAYMENT_UNCONFIRMED
			if pay.PaidHeight != 0 && pay.OnChainEvent.IsZero() {
				// payment is on-chain.
				// notify BUS listeners.
				msg := giga.PaymentEvent{
					PaymentID: pay.ID,
					ForeignID: acc.ForeignID,
					AccountID: acc.Address,
					PayTo:     pay.PayTo,
					Total:     pay.Total,
					TxID:      pay.PaidTxID,
				}
				event := giga.PAYMENT_ON_CHAIN
				unique_id := fmt.Sprintf("POC-%d-%d", cursor, num_pay+n)
				err = b.bus.Send(event, msg, unique_id)
				if err != nil {
					log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
					return err
				}
				// err = tx.MarkPaymentEventSent(pay.ID, event)
				// if err != nil {
				// 	log.Printf("BalanceKeeper: MarkInvoiceEventSent '%s': %v\n", id, err)
				// 	return err
				// }
				b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %v in %s\n", event, pay.ID, id))
			} else if pay.ConfirmedHeight != 0 && pay.ConfirmedEvent.IsZero() {
				// payment is confirmed.
				msg := giga.PaymentEvent{
					PaymentID: pay.ID,
					ForeignID: acc.ForeignID,
					AccountID: acc.Address,
					PayTo:     pay.PayTo,
					Total:     pay.Total,
					TxID:      pay.PaidTxID,
				}
				event := giga.PAYMENT_CONFIRMED
				unique_id := fmt.Sprintf("PCC-%d-%d", cursor, num_pay+n)
				err = b.bus.Send(event, msg, unique_id)
				if err != nil {
					log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
					return err
				}
				// err = tx.MarkInvoiceEventSent(pay.ID, event)
				// if err != nil {
				// 	log.Printf("BalanceKeeper: MarkInvoiceEventSent '%s': %v\n", id, err)
				// 	return err
				// }
				b.bus.Send(giga.SYS_MSG, fmt.Sprintf("BalanceKeeper: %s: %v in %s\n", event, pay.ID, id))
			}
		}
		num_pay += len(payments)
		pay_c = new_pay_c
	}
	log.Printf("BalanceKeeper: checked %d payments: %s\n", num_pay, id)
	return nil
}

func (b *BalanceKeeper) beginStoreTxn() (tx giga.StoreTransaction) {
	for {
		tx, err := b.store.Begin()
		if err != nil {
			log.Println("BalanceKeeper: store.Begin:", err)
			b.sleepForRetry(err, 0)
			continue // retry.
		}
		b.tx = tx // for shutdown.
		return tx
	}
}

func (b *BalanceKeeper) fetchServiceCursor(name string) int64 {
	for {
		cursor, err := b.store.GetServiceCursor(name)
		if err != nil {
			log.Println("BalanceKeeper: GetServiceCursor:", err)
			b.sleepForRetry(err, 0)
			continue // retry.
		}
		return cursor
	}
}

func (b *BalanceKeeper) sleepForRetry(err error, delay time.Duration) {
	if delay == 0 {
		delay = RETRY_DELAY
		if giga.IsDBConflictError(err) {
			delay = CONFLICT_DELAY
		}
	}
	select {
	case <-b.stop:
		panic("shutdown")
	case <-time.After(delay):
		return
	}
}
