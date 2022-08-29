package store

import (
	"database/sql"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

var SETUP_SQL string = `
CREATE TABLE IF NOT EXISTS account (
	foreign_id TEXT NOT NULL UNIQUE,
	address TEXT NOT NULL UNIQUE,
	privkey TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS invoice (
	account_address TEXT NOT NULL,
	vendor TEXT
);
`

// interface guard ensures SQLite implements giga.PaymentsStore
var _ giga.Store = SQLite{}

type SQLite struct {
	db *sql.DB
}

func (s SQLite) MarkInvoiceAsPaid(id giga.Address) error {
	//TODO implement me
	panic("implement me")
}

func (s SQLite) GetPendingInvoices() (<-chan giga.Invoice, error) {
	//TODO implement me
	panic("implement me")
}

// NewSQLite returns a giga.PaymentsStore implementor that uses sqlite
func NewSQLite(fileName string) (SQLite, error) {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		return SQLite{}, err
	}
	// init tables / indexes
	_, err = db.Exec(SETUP_SQL)
	if err != nil {
		return SQLite{}, err
	}

	return SQLite{db}, nil
}

// Defer this until shutdown
func (s SQLite) Close() {
	s.db.Close()
}

func (s SQLite) StoreInvoice(inv giga.Invoice) error {
	tx, err := s.db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := tx.Prepare("insert into invoice(account_address, vendor) values(?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(inv.ID, inv.Vendor)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (s SQLite) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	row := s.db.QueryRow("SELECT account_address, vendor FROM invoice WHERE account_address = ?", addr)
	var id giga.Address
	var vendor string
	err := row.Scan(&id, &vendor)
	if err == sql.ErrNoRows {
		return giga.Invoice{}, err
	}
	if err != nil {
		log.Fatal(err)
	}
	return giga.Invoice{id, vendor, []giga.Item{}}, nil
}

func (s SQLite) StoreAccount(acc giga.Account) error {
	tx, err := s.db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := tx.Prepare("insert into account(foreign_id, address, privkey) values(?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(acc.ForeignID, acc.Address, acc.Privkey)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (s SQLite) GetAccount(foreignID string) (giga.Account, error) {
	row := s.db.QueryRow("SELECT foreign_id, address, privkey FROM account WHERE foreign_id = ?", foreignID)

	var foreign_id string
	var address giga.Address
	var privkey giga.Privkey
	err := row.Scan(&foreign_id, &address, &privkey)
	if err == sql.ErrNoRows {
		return giga.Account{}, err
	}
	if err != nil {
		log.Fatal(err)
	}
	return giga.Account{address, privkey, foreign_id}, nil
}

func (s SQLite) GetAccountByAddress(id giga.Address) (giga.Account, error) {
	// TODO: make the sql query
	return giga.Account{}, nil
}
