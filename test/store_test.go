package test

import (
	"fmt"
	"testing"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	sqlite "github.com/dogecoinfoundation/gigawallet/pkg/store"
	"github.com/shopspring/decimal"
)

const addr1 giga.Address = "DHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx1L"
const addr2 giga.Address = "DHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx2L"
const addr3 giga.Address = "DHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx3L"

var pi giga.CoinAmount = decimal.RequireFromString("3.14159")

func TestStore(t *testing.T) {

	// implementations to test
	stores := map[string]giga.Store{}

	// set up the sqlite store
	s1, err := sqlite.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal("Couldn't open sqlite DB")
	}

	stores["sqlite"] = s1

	for storeName, store := range stores {

		//create a unique test name
		n := func(s string) string {
			return fmt.Sprintf("Store-%s-%s", storeName, s)
		}

		t.Run(n("Account"), func(t *testing.T) {

			tx, err := store.Begin()
			if err != nil {
				t.Fatal(n("establish transaction"), err)
			}

			// Test Account creation
			account := giga.Account{
				ForeignID: "test",
				Address:   addr1,
			}
			err = tx.CreateAccount(account)
			if err != nil {
				t.Fatal(n("CreateAccount"), err)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(n("commit transaction"), err)
			}

			// Test GetAccount
			retrievedAccount, err := store.GetAccount(account.ForeignID)
			if err != nil {
				t.Fatal(n("GetAccount"), err)
			}

			// Test UpdateAccount
			tx, err = store.Begin()
			if err != nil {
				t.Fatal(n("establish transaction"), err)
			}

			updatedAccount := retrievedAccount
			updatedAccount.PayoutAddress = addr2
			err = tx.UpdateAccount(updatedAccount)
			if err != nil {
				t.Fatal(n("UpdateAccount"), err)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(n("commit transaction"), err)
			}

			// Test GetAccount
			retrievedAccount, err = store.GetAccount(account.ForeignID)
			if err != nil {
				t.Fatal(n("GetAccount"), err)
			}

			if retrievedAccount.PayoutAddress != addr2 {
				t.Fatal(n("verify updateAccount failed"), retrievedAccount)
			}

		})

		t.Run(n("Invoice"), func(t *testing.T) {
			tx, err := store.Begin()
			if err != nil {
				t.Fatal(n("establish transaction"), err)
			}

			// Test Invoice creation
			invoice := giga.Invoice{
				ID:      addr3,
				Account: addr1,
				Created: time.Now(),
				Items: []giga.Item{
					giga.Item{
						Type:     "item",
						Name:     "foo",
						Value:    pi,
						Quantity: 1,
					},
				},
			}
			err = tx.StoreInvoice(invoice)
			if err != nil {
				t.Fatal(n("StoreInvoice"), err)
			}

			// Test GetInvoice
			retrievedInvoice, err := tx.GetInvoice(invoice.ID)
			if err != nil {
				t.Fatal(n("GetInvoice"), err)
			}

			//Test Invoice.CalcTotal
			if retrievedInvoice.CalcTotal() != pi {
				t.Fatal(n("Invoice.CalcTotal"), retrievedInvoice.CalcTotal())
			}

			// Test ListInvoices
			invoices, counter, err := tx.ListInvoices(invoice.Account, 0, 10)
			if err != nil {
				t.Fatal(n("ListInvoice"), err)
			}

			if len(invoices) != 1 {
				t.Fatal(n("Unexpected length of invoices"), invoices, counter)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(n("commit transaction"), err)
			}
		})

	}
}
