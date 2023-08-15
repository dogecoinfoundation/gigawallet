package services

import (
	"context"
	"log"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

const (
	RETRY_DELAY    = 5 * time.Second        // for Database errors.
	CONFLICT_DELAY = 250 * time.Millisecond // for Database conflicts (concurrent transactions)
	BATCH_SIZE     = 10                     // number of Accounts to request at once
)

type BalanceKeeper struct {
	store giga.Store
	stop  chan context.Context
	tx    giga.StoreTransaction // non-nil during a transaction (for shutdown)
}

func NewBalanceKeeper(store giga.Store) BalanceKeeper {
	keeper := BalanceKeeper{
		store: store,
		stop:  nil,
	}
	return keeper
}

func (b BalanceKeeper) runBatch(cursor int64) (int64, error) {
	// tx := b.beginStoreTxn()
	// ids, newCursor, err := tx.ListAccountsModifiedSince(cursor, BATCH_SIZE)
	// if err != nil {
	// 	log.Println("BalanceKeeper: GetServiceCursor:", err)
	// 	return cursor, err
	// }
	// for _, id := range ids {
	// 	log.Printf("BalanceKeeper: account was modified: %s @ %v\n", id, newCursor)
	// }
	// err = b.tx.Commit()
	// if err != nil {
	// 	return cursor, err
	// }
	// return newCursor, nil
	return cursor, nil
}

func (b *BalanceKeeper) beginStoreTxn() (tx giga.StoreTransaction) {
	for {
		log.Println("BalanceKeeper: store.Begin")
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
		cursor := b.fetchServiceCursor("BalanceKeeper")
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
				}
				b.sleepForRetry(nil, 1*time.Second)
				cursor = newCursor // advance the cursor.
			}
		}
	}()
	return nil
}
