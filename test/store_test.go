package test

import (
	"fmt"
	"testing"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	sqlite "github.com/dogecoinfoundation/gigawallet/pkg/store"
)

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
			updatedAccount.PayoutAddress = "123"
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

			if retrievedAccount.PayoutAddress != "123" {
				t.Fatal(n("verify updateAccount failed"), retrievedAccount)
			}

		})
	}
}
