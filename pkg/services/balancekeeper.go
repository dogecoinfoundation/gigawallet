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
	BATCH_SIZE         = 10              // number of Accounts to process at once
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
	ids, newCursor, err := tx.ListAccountsModifiedSince(cursor, BATCH_SIZE)
	if err != nil {
		tx.Rollback()
		log.Println("BalanceKeeper: ListAccountsModifiedSince:", err)
		return cursor, err
	}
	for n, id := range ids {
		err = b.updateAccountBalance(tx, id, cursor, n)
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

func (b BalanceKeeper) updateAccountBalance(tx giga.StoreTransaction, id string, cursor int64, n int) error {
	log.Printf("BalanceKeeper: checking account balance: %s\n", id)
	acc, err := tx.GetAccountByID(id)
	if err != nil {
		log.Printf("BalanceKeeper: GetAccountByID '%s': %v\n", id, err)
		return err
	}
	bal, err := tx.CalculateBalance(giga.Address(id))
	if err != nil {
		log.Printf("BalanceKeeper: CalculateBalance '%s': %v\n", id, err)
		return err
	}
	if !(bal.CurrentBalance.Equals(acc.CurrentBalance) &&
		bal.IncomingBalance.Equals(acc.IncomingBalance) &&
		bal.OutgoingBalance.Equals(acc.OutgoingBalance)) {
		// update the stored account balance.
		err = tx.UpdateAccountBalance(acc.Address, bal)
		if err != nil {
			log.Printf("BalanceKeeper: UpdateAccountBalance '%s': %v\n", id, err)
			return err
		}
		// notify BUS listeners.
		msg := giga.AccountBalanceChange{
			AccountID:       acc.Address,
			ForeignID:       acc.ForeignID,
			CurrentBalance:  bal.CurrentBalance,
			IncomingBalance: bal.IncomingBalance,
			OutgoingBalance: bal.OutgoingBalance,
		}
		unique_id := fmt.Sprintf("BK-%d-%d", cursor, n)
		err = b.bus.Send(giga.ACC_BALANCE_CHANGE, msg, unique_id)
		if err != nil {
			log.Printf("BalanceKeeper: bus error for '%s': %v\n", id, err)
			return err
		}
		log.Printf("BalanceKeeper: updated account balance: %s\n", id)
	}
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
