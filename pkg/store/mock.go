package store

import (
	"errors"
	"fmt"
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

// interface guard ensures Mock implements giga.PaymentsStore
var _ giga.Store = Mock{}

type Mock struct {
	invoices          map[giga.Address]giga.Invoice
	accounts          map[string]giga.Account
	accountsByAddress map[giga.Address]giga.Account
}

// NewMock returns a giga.PaymentsStore implementor that stores orders in memory
func NewMock() Mock {
	return Mock{}
}

func (m Mock) StoreInvoice(invoice giga.Invoice) error {
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m Mock) GetInvoice(id giga.Address) (giga.Invoice, error) {
	v, ok := m.invoices[id]
	if !ok {
		return giga.Invoice{}, errors.New("invoice not found for id " + fmt.Sprint(id))
	}
	return v, nil
}

func (m Mock) StoreAccount(account giga.Account) error {
	m.accounts[account.ForeignID] = account
	m.accountsByAddress[account.Address] = account
	return nil
}

func (m Mock) GetAccount(foreignID string) (giga.Account, error) {
	v, ok := m.accounts[foreignID]
	if !ok {
		return giga.Account{}, errors.New("account not found for foreignID " + fmt.Sprint(foreignID))
	}
	return v, nil
}

func (m Mock) GetAccountByAddress(id giga.Address) (giga.Account, error) {
	v, ok := m.accountsByAddress[id]
	if !ok {
		return giga.Account{}, errors.New("account not found for id " + fmt.Sprint(id))
	}
	return v, nil
}
