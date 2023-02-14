package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/shopspring/decimal"

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
	invoice_address TEXT NOT NULL,
	account_address TEXT NOT NULL,
	txn_id TEXT NOT NULL,
	vendor TEXT NOT NULL,
	items TEXT NOT NULL,
	key_index INTEGER NOT NULL,
	block_id TEXT NOT NULL,
	confirmations INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS utxo (
	account_address TEXT NOT NULL,
	txn_id TEXT NOT NULL,
	vout INTEGER NOT NULL,
	status TEXT NOT NULL,
	value TEXT NOT NULL,
	script_type TEXT NOT NULL,
	script_address TEXT NOT NULL
);
`

// interface guard ensures SQLite implements giga.PaymentsStore
var _ giga.Store = SQLite{}

type SQLite struct {
	db *sql.DB
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
		return SQLite{}, dbErr(err, "opening database")
	}
	// init tables / indexes
	_, err = db.Exec(SETUP_SQL)
	if err != nil {
		return SQLite{}, dbErr(err, "creating database schema")
	}

	return SQLite{db}, nil
}

// Defer this until shutdown
func (s SQLite) Close() {
	s.db.Close()
}

func (s SQLite) StoreInvoice(inv giga.Invoice) error {
	// replaced with Commit API.
	return s.Commit([]any{
		giga.UpsertInvoice{
			Invoice: inv,
		},
	})
}

func (s SQLite) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	row := s.db.QueryRow("SELECT invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE invoice_address = ?", addr)
	var id giga.Address
	var account giga.Address
	var tx_id string
	var vendor string
	var items_json string
	var key_index uint32
	var block_id string
	var confirmations int32
	err := row.Scan(&id, &account, &tx_id, &vendor, &items_json, &key_index, &block_id, &confirmations)
	if err == sql.ErrNoRows {
		return giga.Invoice{}, giga.NewErr(giga.NotFound, "invoice not found: %v", addr)
	}
	if err != nil {
		return giga.Invoice{}, dbErr(err, "GetInvoice: row.Scan")
	}
	var items []giga.Item
	err = json.Unmarshal([]byte(items_json), &items)
	if err != nil {
		return giga.Invoice{}, dbErr(err, "GetInvoice: json.Unmarshal")
	}
	return giga.Invoice{
		ID:            id,
		Account:       account,
		TXID:          tx_id,
		Vendor:        vendor,
		Items:         items,
		KeyIndex:      key_index,
		BlockID:       block_id,
		Confirmations: confirmations,
	}, nil
}

func (s SQLite) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// MUST order by key_index (or sqlite OID) to support the cursor API:
	// we need a way to resume the query next time from whatever next_cursor we return,
	// and the aggregate result SHOULD be stable even as the DB is modified.
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := s.db.Query("SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ? AND key_index >= ? ORDER BY key_index LIMIT ?", account, cursor, limit)
	if err != nil {
		return nil, 0, dbErr(err, "ListInvoices: querying invoices")
	}
	defer rows.Close()
	for rows.Next() {
		inv := giga.Invoice{Account: account}
		var items_json string
		err := rows.Scan(&inv.ID, &inv.TXID, &inv.Vendor, &items_json, &inv.KeyIndex, &inv.BlockID, &inv.Confirmations)
		if err != nil {
			return nil, 0, dbErr(err, "ListInvoices: scanning invoice row")
		}
		err = json.Unmarshal([]byte(items_json), &inv.Items)
		if err != nil {
			return nil, 0, dbErr(err, "ListInvoices: unmarshalling json")
		}
		items = append(items, inv)
		after_this := int(inv.KeyIndex) + 1 // XXX assumes non-hardened HD Key! (from uint32)
		if after_this > next_cursor {
			next_cursor = after_this // NB. starting cursor for next call
		}
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, 0, dbErr(err, "ListInvoices: querying invoices")
	}
	if rows_found < limit {
		// in this backend, we know there are no more rows to follow.
		next_cursor = 0 // meaning "end of query results"
	}
	return
}

func (s SQLite) StoreAccount(acc giga.Account) error {
	// replaced with Commit API.
	return s.Commit([]any{
		giga.UpsertAccount{
			Account: acc,
		},
	})
}

func (s SQLite) GetAccount(foreignID string) (giga.Account, error) {
	row := s.db.QueryRow("SELECT foreign_id, address, privkey, next_int_key, next_ext_key FROM account WHERE foreign_id = ?", foreignID)
	var acc giga.Account
	err := row.Scan(&acc.ForeignID, &acc.Address, &acc.Privkey, &acc.NextInternalKey, &acc.NextExternalKey)
	if err == sql.ErrNoRows {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %s", foreignID)
	}
	if err != nil {
		return giga.Account{}, dbErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (s SQLite) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	rows_found := 0
	rows, err := s.db.Query("SELECT txn_id, vout, value, script_type, script_address FROM utxo WHERE account_address = ? AND status = 'c'", account)
	if err != nil {
		return nil, dbErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	defer rows.Close()
	for rows.Next() {
		utxo := giga.UTXO{Account: account, Status: "c"}
		var value string
		err := rows.Scan(&utxo.TxnID, &utxo.VOut, &value, &utxo.ScriptType, &utxo.ScriptAddress)
		if err != nil {
			return nil, dbErr(err, "GetAllUnreservedUTXOs: scanning UTXO row")
		}
		utxo.Value, err = decimal.NewFromString(value)
		if err != nil {
			return nil, dbErr(err, fmt.Sprintf("GetAllUnreservedUTXOs: invalid decimal value in UTXO database: %v", value))
		}
		result = append(result, utxo)
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, dbErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	return
}

func (s SQLite) Commit(updates []any) error {
	tx, err := s.db.Begin()
	if err != nil {
		return dbErr(err, "Beginning transaction")
	}

	for _, update := range updates {
		switch u := update.(type) {
		case giga.UpsertAccount:
			return s.createAccount(tx, u.Account)
		case giga.UpdateAccountNextExternal:
			return s.updateAccountNextExternal(tx, u.Address, u.KeyIndex)
		case giga.UpsertInvoice:
			return s.createInvoice(tx, u.Invoice)
		case giga.MarkInvoiceAsPaid:
			return s.markInvoiceAsPaid(tx, u.InvoiceID)
		}
	}

	err = tx.Commit()
	if err != nil {
		return dbErr(err, "Committing transaction")
	}

	return nil
}

func (s SQLite) createAccount(tx *sql.Tx, acc giga.Account) error {
	stmt, err := tx.Prepare("insert into account(foreign_id, address, privkey, next_int_key, next_ext_key) values(?, ?, ?, ?, ?)")
	if err != nil {
		return dbErr(err, "createAccount: preparing insert")
	}
	defer stmt.Close()
	_, err = stmt.Exec(acc.ForeignID, acc.Address, acc.Privkey, acc.NextInternalKey, acc.NextExternalKey)
	if err != nil {
		return dbErr(err, "createAccount: executing insert")
	}
	return nil
}

func (s SQLite) createInvoice(tx *sql.Tx, inv giga.Invoice) error {
	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		return dbErr(err, "createInvoice: json.Marshal items")
	}

	stmt, err := tx.Prepare("insert into invoice(invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations) values(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return dbErr(err, "createInvoice: tx.Prepare insert")
	}
	defer stmt.Close()

	_, err = stmt.Exec(inv.ID, inv.Account, inv.TXID, inv.Vendor, string(items_b), inv.KeyIndex, inv.BlockID, inv.Confirmations)
	if err != nil {
		return dbErr(err, "createInvoice: stmt.Exec insert")
	}

	return s.updateAccountNextExternal(tx, inv.Account, inv.KeyIndex)
}

func (s SQLite) updateAccountNextExternal(tx *sql.Tx, account giga.Address, keyIndex uint32) error {
	stmt, err := tx.Prepare("update account set next_ext_key = MAX(next_ext_key, ?) where address = ?")
	if err != nil {
		return dbErr(err, "StoreInvoice: tx.Prepare update")
	}
	defer stmt.Close()

	// update Account to mark KeyIndex as used.
	res, err := stmt.Exec(keyIndex+1, account)
	if err != nil {
		return dbErr(err, "StoreInvoice: update.Exec")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return dbErr(err, "StoreInvoice: res.RowsAffected")
	}
	if num_rows < 1 {
		return giga.NewErr(giga.NotFound, "unknown account: %s", account)
	}
	return nil
}

func (s SQLite) markInvoiceAsPaid(tx *sql.Tx, address giga.Address) error {
	stmt, err := tx.Prepare("update invoices set confirmations = ? where invoice_address = ?")
	if err != nil {
		return dbErr(err, "markInvoiceAsPaid: preparing update")
	}
	defer stmt.Close()
	_, err = stmt.Exec(1, address)
	if err != nil {
		return dbErr(err, "markInvoiceAsPaid: executing update")
	}
	return nil
}

func dbErr(err error, where string) error {
	return giga.NewErr(giga.NotAvailable, "SQLite error: %s: %v", where, err)
}
