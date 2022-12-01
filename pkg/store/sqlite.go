package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	_ "github.com/mattn/go-sqlite3"
)

var SETUP_SQL string = `
CREATE TABLE IF NOT EXISTS account (
	foreign_id TEXT NOT NULL UNIQUE,
	address TEXT NOT NULL UNIQUE,
	privkey TEXT NOT NULL,
	next_int_key INTEGER NOT NULL,
	next_ext_key INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS invoice (
	account_address TEXT NOT NULL,
	txn_id TEXT NOT NULL,
	vendor TEXT NOT NULL,
	items TEXT NOT NULL,
	key_index INTEGER NOT NULL,
	block_id TEXT NOT NULL,
	confirmations INTEGER NOT NULL
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
	log.Print("GetPendingInvoices: not implemented")
	return make(chan giga.Invoice), nil
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

	stmt, err := tx.Prepare("insert into invoice(account_address, txn_id, vendor, items, key_index, block_id, confirmations) values(?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		log.Fatal(err)
	}

	_, err = stmt.Exec(inv.ID, inv.TXID, inv.Vendor, string(items_b), inv.KeyIndex, inv.BlockID, inv.Confirmations)
	if err != nil {
		log.Fatal(err)
	}

	update, err := tx.Prepare("update account set next_ext_key = MAX(next_ext_key, ?) where foreign_id = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	// update Account to mark KeyIndex as used.
	res, err := update.Exec(inv.Account, inv.KeyIndex+1)
	if err != nil {
		log.Fatal(err)
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	if num_rows < 1 {
		return fmt.Errorf("unknown account: %s", inv.Account)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (s SQLite) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	row := s.db.QueryRow("SELECT account_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ?", addr)
	var id giga.Address
	var tx_id string
	var vendor string
	var items_json string
	var key_index uint32
	var block_id string
	var confirmations int32
	err := row.Scan(&id, &tx_id, &vendor, &items_json, &key_index, &block_id, &confirmations)
	if err == sql.ErrNoRows {
		return giga.Invoice{}, err
	}
	if err != nil {
		log.Fatal(err)
	}
	var items []giga.Item
	err = json.Unmarshal([]byte(items_json), &items)
	if err != nil {
		log.Fatal(err)
	}
	return giga.Invoice{
		ID:            id,
		TXID:          tx_id,
		Vendor:        vendor,
		Items:         items,
		KeyIndex:      key_index,
		BlockID:       block_id,
		Confirmations: confirmations,
	}, nil
}

func (s SQLite) StoreAccount(acc giga.Account) error {
	tx, err := s.db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := tx.Prepare("insert into account(foreign_id, address, privkey, next_int_key, next_ext_key) values(?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(acc.ForeignID, acc.Address, acc.Privkey, acc.NextInternalKey, acc.NextExternalKey)
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
	row := s.db.QueryRow("SELECT foreign_id, address, privkey, next_int_key, next_ext_key FROM account WHERE foreign_id = ?", foreignID)

	var foreign_id string
	var address giga.Address
	var privkey giga.Privkey
	var next_int_key uint32
	var next_ext_key uint32
	err := row.Scan(&foreign_id, &address, &privkey, &next_int_key, &next_ext_key)
	if err == sql.ErrNoRows {
		return giga.Account{}, err
	}
	if err != nil {
		log.Fatal(err)
	}
	return giga.Account{
		Address:         address,
		Privkey:         privkey,
		ForeignID:       foreign_id,
		NextInternalKey: next_int_key,
		NextExternalKey: next_ext_key,
	}, nil
}

func (s SQLite) GetAccountByAddress(id giga.Address) (giga.Account, error) {
	// TODO: make the sql query
	return giga.Account{}, nil
}
