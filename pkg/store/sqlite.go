package store

import (
	"database/sql"
	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

// interface guard ensures SQLite implements giga.PaymentsStore
var _ giga.Store = SQLite{}

type SQLite struct {
	db *sql.DB
}

// NewSQLite returns a giga.PaymentsStore implementor that uses sqlite
func NewSQLite(fileName string) (SQLite, error) {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		return SQLite{}, err
	}
	return SQLite{db}, nil
}

func (s SQLite) StoreInvoice(order giga.Invoice) error {
	// TODO: make the sql query
	return nil
}

func (s SQLite) GetInvoice(id giga.Address) (giga.Invoice, error) {
	// TODO: make the sql query
	return giga.Invoice{}, nil
}

func (s SQLite) StoreAccount(account giga.Account) error {
	// TODO: make the sql query
	return nil
}

func (s SQLite) GetAccount(pubkey string) (giga.Account, error) {
	// TODO: make the sql query
	return giga.Account{}, nil
}
