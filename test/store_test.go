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
	// :memory: or postgres://postgres:@localhost/testdb?sslmode=disable
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
			defer tx.Rollback()

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
			defer tx.Rollback()

			updatedAccount := retrievedAccount
			updatedAccount.PayoutAddress = addr2
			err = tx.UpdateAccountConfig(updatedAccount)
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
			defer tx.Rollback()

			// Test Invoice creation
			invoice := giga.Invoice{
				ID:      addr3,
				Account: addr1,
				Created: time.Now(),
				Items: []giga.Item{
					{
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
			if retrievedInvoice.CalcTotal().Cmp(pi) != 0 {
				t.Fatal(n("Invoice.CalcTotal"), retrievedInvoice.CalcTotal(), pi)
			}

			// Test ListInvoices
			invoices, counter, err := tx.ListInvoices(invoice.Account, 0, 10)
			if err != nil {
				t.Fatal(n("ListInvoice"), err)
			}

			if len(invoices) != 1 {
				t.Fatal(n("Unexpected length of invoices"), invoices, counter)
			}

			// Create a bunch of invoices to test pagination
			for i := 0; i < 18; i++ {
				// Test Invoice creation
				invoice := giga.Invoice{
					ID:       giga.Address(fmt.Sprintf("DHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx%d", i)),
					KeyIndex: uint32(i),
					Account:  addr1,
					Created:  time.Now(),
					Items: []giga.Item{
						{
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
			}

			// iterate using the counter, should get next 10
			invoices2, counter2, err := tx.ListInvoices(invoice.Account, 0, 10)
			if err != nil {
				t.Fatal(n("ListInvoice"), err)
			}

			// should have 10 invoices
			if len(invoices2) != 10 {
				t.Fatal(n("Unexpected length of invoices"), invoices2, counter2)
			}

			// counter should be not 0
			if counter2 == 0 {
				t.Fatal(n("Counter should be non zero"), counter)
			}

			// iterate using the counter, should get next 9
			invoices3, counter3, err := tx.ListInvoices(invoice.Account, counter2, 10)
			if err != nil {
				t.Fatal(n("ListInvoice"), err)
			}

			// should have 10 invoices
			if len(invoices3) != 9 {
				t.Fatal(n("Unexpected length of invoices"), len(invoices3))
			}

			// counter should be not 0
			if counter3 != 0 {
				t.Fatal(n("Counter should be non zero"), counter3)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(n("commit transaction"), err)
			}
		})

		t.Run(n("Payment"), func(t *testing.T) {
			tx, err := store.Begin()
			if err != nil {
				t.Fatal(n("establish transaction"), err)
			}
			defer tx.Rollback()

			// Test Payment creation
			payTo := []giga.PayTo{
				{
					Amount:           decimal.NewFromInt(100),
					PayTo:            addr2,
					DeductFeePercent: decimal.NewFromInt(100),
				},
			}
			pay, err := tx.CreatePayment(addr1, payTo, decimal.NewFromInt(100), decimal.NewFromInt(1))
			if err != nil {
				t.Fatal(n("CreatePayment"), err)
			}

			// Test GetPayment
			retrievedPayment, err := tx.GetPayment(addr1, pay.ID)
			if err != nil {
				t.Fatal(n("GetPayment"), err)
			}
			if retrievedPayment.AccountAddress != addr1 || len(retrievedPayment.PayTo) != 1 {
				t.Fatal(n("GetPayment: wrong payment address or len"))
			}
			if !retrievedPayment.Total.Equals(decimal.NewFromInt(100)) || !retrievedPayment.Fee.Equals(decimal.NewFromInt(1)) {
				t.Fatal(n("GetPayment: wrong payment values"))
			}
			paidTo := retrievedPayment.PayTo[0]
			if !paidTo.Amount.Equals(decimal.NewFromInt(100)) || paidTo.PayTo != addr2 || !paidTo.DeductFeePercent.Equals(decimal.NewFromInt(100)) {
				t.Fatal(n("GetPayment: wrong PayTo details"))
			}

			// Test ListPayments
			payments, counter, err := tx.ListPayments(addr1, 0, 10)
			if err != nil {
				t.Fatal(n("ListInvoice"), err)
			}

			if len(payments) != 1 {
				t.Fatal(n("Unexpected length of payments"), payments, counter)
			}

			// Create a bunch of payments to test pagination
			for i := 0; i < 18; i++ {
				// Test Payment creation
				payTo := []giga.PayTo{
					{
						Amount:           decimal.NewFromInt(int64(i) + 1),
						PayTo:            giga.Address(fmt.Sprintf("DHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx%d", i)),
						DeductFeePercent: decimal.Zero,
					},
				}
				_, err := tx.CreatePayment(addr1, payTo, decimal.NewFromInt(100), decimal.NewFromInt(1))
				if err != nil {
					t.Fatal(n("CreatePayment"), err)
				}
			}

			// iterate using the counter, should get next 10
			payments2, counter2, err := tx.ListPayments(addr1, 0, 10)
			if err != nil {
				t.Fatal(n("ListPayments"), err)
			}

			// should have 10 payments
			if len(payments2) != 10 {
				t.Fatal(n("Unexpected length of payments"), payments2, counter2)
			}

			// counter should be not 0
			if counter2 == 0 {
				t.Fatal(n("Counter should be non zero"), counter)
			}

			// iterate using the counter, should get next 9
			payments3, counter3, err := tx.ListPayments(addr1, counter2, 10)
			if err != nil {
				t.Fatal(n("ListPayments"), err)
			}

			// should have 9 payments
			if len(payments3) != 9 {
				t.Fatal(n("Unexpected length of payments"), len(payments3))
			}

			// counter should be not 0
			if counter3 != 0 {
				t.Fatal(n("Counter should be non zero"), counter3)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(n("commit transaction"), err)
			}
		})

	}
}
