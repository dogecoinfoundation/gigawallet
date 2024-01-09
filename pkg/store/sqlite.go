package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const SETUP_SQL string = `
CREATE TABLE IF NOT EXISTS account (
	address TEXT NOT NULL PRIMARY KEY,
	foreign_id TEXT NOT NULL UNIQUE,
	privkey TEXT NOT NULL,
	next_int_key INTEGER NOT NULL,
	next_ext_key INTEGER NOT NULL,
	next_pool_int INTEGER NOT NULL,
	next_pool_ext INTEGER NOT NULL,
	payout_address TEXT NOT NULL,
	payout_threshold NUMERIC(18,8) NOT NULL,
	payout_frequency TEXT NOT NULL,
	current_balance NUMERIC(18,8) NOT NULL DEFAULT 0,
	incoming_balance NUMERIC(18,8) NOT NULL DEFAULT 0,
	outgoing_balance NUMERIC(18,8) NOT NULL DEFAULT 0,
	chain_seq INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS account_foreign_i ON account (foreign_id);

CREATE TABLE IF NOT EXISTS account_address (
	address TEXT NOT NULL PRIMARY KEY,
	key_index INTEGER NOT NULL,
	is_internal BOOLEAN NOT NULL,
	account_address TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS account_address_i ON account_address (account_address);

CREATE TABLE IF NOT EXISTS invoice (
	invoice_address TEXT NOT NULL PRIMARY KEY,
	account_address TEXT NOT NULL,
	items TEXT NOT NULL,
	total NUMERIC(18,8) NOT NULL,
	key_index INTEGER NOT NULL,
	confirmations INTEGER NOT NULL,
	created DATETIME NOT NULL,
	incoming_amount NUMERIC(18,8),
	paid_amount NUMERIC(18,8),
	last_incoming NUMERIC(18,8),
	last_paid NUMERIC(18,8),
	paid_height INTEGER,
	block_id TEXT,
	paid_event DATETIME
);
CREATE INDEX IF NOT EXISTS invoice_account_i ON invoice (account_address);

CREATE TABLE IF NOT EXISTS payment (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_address TEXT NOT NULL,
	created DATETIME NOT NULL,
	total NUMERIC(18,8) NOT NULL,
	fee NUMERIC(18,8) NOT NULL,
	paid_txid TEXT,
	paid_height INTEGER,
	confirmed_height INTEGER,
	on_chain_event DATETIME,
	confirmed_event DATETIME,
	unconfirmed_event DATETIME
);

CREATE INDEX IF NOT EXISTS payment_account_i ON payment (account_address);
CREATE INDEX IF NOT EXISTS payment_txid_i ON payment (paid_txid);
CREATE INDEX IF NOT EXISTS payment_paid_height_i ON payment (paid_height);

CREATE TABLE IF NOT EXISTS output (
	payment_id INTEGER NOT NULL,
	vout INTEGER NOT NULL,
	pay_to TEXT NOT NULL,
	amount NUMERIC(18,8) NOT NULL,
	deduct_fee_percent NUMERIC(18,8) NOT NULL,
	PRIMARY KEY (payment_id, vout)
);

CREATE TABLE IF NOT EXISTS utxo (
	txn_id TEXT NOT NULL,
	vout INTEGER NOT NULL,
	value NUMERIC(18,8) NOT NULL,
	script TEXT NOT NULL,
	script_type TEXT NOT NULL,
	script_address TEXT NOT NULL,
	account_address TEXT NOT NULL,
	key_index INTEGER NOT NULL,
	is_internal BOOLEAN NOT NULL,
	added_height INTEGER,
	spendable_height INTEGER,
	spending_height INTEGER,
	spent_height INTEGER,
	spend_txid TEXT,
	PRIMARY KEY (txn_id, vout)
);
CREATE INDEX IF NOT EXISTS utxo_account_i ON utxo (account_address);
CREATE INDEX IF NOT EXISTS utxo_added_i ON utxo (added_height, spendable_height);
CREATE INDEX IF NOT EXISTS utxo_spent_i ON utxo (spending_height, spent_height);

CREATE TABLE IF NOT EXISTS chainstate (
	root_hash TEXT NOT NULL,
	first_height INTEGER NOT NULL,
	best_hash TEXT NOT NULL,
	best_height INTEGER NOT NULL,
	next_seq INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS services (
	name TEXT NOT NULL PRIMARY KEY,
	cursor INTEGER NOT NULL
);
`

/****************** SQLiteStore implements giga.Store ********************/
var _ giga.Store = SQLiteStore{}

type SQLiteStore struct {
	db         *sql.DB
	isPostgres bool
}

// The common read-only parts of sql.DB and sql.Tx interfaces, so we can pass either
// one to some helper functions (for methods that appear on both SQLiteStore and
// SQLiteStoreTransaction)
type Queryable interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// NewSQLiteStore returns a giga.PaymentsStore implementor that uses sqlite
func NewSQLiteStore(fileName string) (giga.Store, error) {
	backend := "sqlite3"
	if strings.HasPrefix(fileName, "postgres://") {
		// "postgres://user:password@localhost/gigawallet"
		backend = "postgres"
	}
	db, err := sql.Open(backend, fileName)
	store := SQLiteStore{db: db, isPostgres: (backend == "postgres")}
	if err != nil {
		return SQLiteStore{}, store.dbErr(err, "opening database")
	}
	setup_sql := SETUP_SQL
	if backend == "sqlite3" {
		// limit concurrent access until we figure out a way to start transactions
		// with the BEGIN CONCURRENT statement in Go.
		db.SetMaxOpenConns(1)
		// WAL mode provides more concurrency, although not necessary with above.
		// _, err = db.Exec("PRAGMA journal_mode=WAL")
		// if err != nil {
		// 	return SQLiteStore{}, store.dbErr(err, "creating database schema")
		// }
	} else {
		setup_sql = strings.ReplaceAll(setup_sql, "INTEGER PRIMARY KEY AUTOINCREMENT", "SERIAL")
		setup_sql = strings.ReplaceAll(setup_sql, "DATETIME", "TIMESTAMP WITH TIME ZONE")
	}
	// init tables / indexes
	_, err = db.Exec(setup_sql)
	if err != nil {
		return SQLiteStore{}, store.dbErr(err, "creating database schema")
	}
	return store, nil
}

// Defer this until shutdown
func (s SQLiteStore) Close() {
	s.db.Close()
}

func (s SQLiteStore) Begin() (giga.StoreTransaction, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return &SQLiteStoreTransaction{}, err
	}
	return &SQLiteStoreTransaction{tx: tx, finality: false, store: s}, nil
}

func (s SQLiteStore) GetAccount(foreignID string) (giga.Account, error) {
	return s.getAccountCommon(s.db, foreignID, true /*isForeignKey*/)
}

func (s SQLiteStore) CalculateBalance(accountID giga.Address) (giga.AccountBalance, error) {
	return s.calculateBalanceCommon(s.db, accountID)
}

func (s SQLiteStore) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	return s.getInvoiceCommon(s.db, addr)
}

func (s SQLiteStore) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	return s.listInvoicesCommon(s.db, account, cursor, limit)
}

func (s SQLiteStore) GetPayment(account giga.Address, id int64) (giga.Payment, error) {
	return s.getPaymentCommon(s.db, account, id)
}

func (s SQLiteStore) ListPayments(account giga.Address, cursor int64, limit int) (items []giga.Payment, next_cursor int64, err error) {
	return s.listPaymentsCommon(s.db, account, cursor, limit)
}

func (s SQLiteStore) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	return s.getAllUnreservedUTXOsCommon(s.db, account)
}

func (s SQLiteStore) GetChainState() (giga.ChainState, error) {
	return s.getChainStateCommon(s.db)
}

func (s SQLiteStore) getChainStateCommon(tx Queryable) (giga.ChainState, error) {
	row := tx.QueryRow("SELECT best_hash, best_height, root_hash, first_height, next_seq FROM chainstate")
	var state giga.ChainState
	err := row.Scan(&state.BestBlockHash, &state.BestBlockHeight, &state.RootHash, &state.FirstHeight, &state.NextSeq)
	if err == sql.ErrNoRows {
		// MUST detect this error to fulfil the API contract.
		return giga.ChainState{}, giga.NewErr(giga.NotFound, "chainstate not found")
	}
	if err != nil {
		return giga.ChainState{}, s.dbErr(err, "GetChainState: row.Scan")
	}
	return state, nil
}

func (s SQLiteStore) GetServiceCursor(name string) (cursor int64, err error) {
	row := s.db.QueryRow("SELECT cursor FROM services WHERE name=$1", name)
	err = row.Scan(&cursor)
	if err == sql.ErrNoRows {
		return 0, nil // new service; start from cursor=0
	}
	if err != nil {
		return 0, s.dbErr(err, "GetServiceCursor: row.Scan")
	}
	return
}

func (s SQLiteStore) getAccountCommon(tx Queryable, accountKey string, isForeignKey bool) (giga.Account, error) {
	// Used to fetch an Account by ID (Address) or by ForeignID.
	query := "SELECT foreign_id,address,privkey,next_int_key,next_ext_key,next_pool_int,next_pool_ext,payout_address,payout_threshold,payout_frequency,current_balance,incoming_balance,outgoing_balance FROM account WHERE "
	if isForeignKey {
		query += "foreign_id = $1"
	} else {
		query += "address = $1"
	}
	row := tx.QueryRow(query, accountKey)
	var acc giga.Account
	err := row.Scan(
		&acc.ForeignID, &acc.Address, &acc.Privkey,
		&acc.NextInternalKey, &acc.NextExternalKey,
		&acc.NextPoolInternal, &acc.NextPoolExternal,
		&acc.PayoutAddress, &acc.PayoutThreshold, &acc.PayoutFrequency, // common (see updateAccount)
		&acc.CurrentBalance, &acc.IncomingBalance, &acc.OutgoingBalance) // not in updateAccount.
	if err == sql.ErrNoRows {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %s", accountKey)
	}
	if err != nil {
		return giga.Account{}, s.dbErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (s SQLiteStore) calculateBalanceCommon(tx Queryable, accountID giga.Address) (bal giga.AccountBalance, err error) {
	row := tx.QueryRow(`
SELECT COALESCE((SELECT SUM(value) FROM utxo WHERE added_height IS NOT NULL AND spendable_height IS NULL AND account_address=$1),0),
COALESCE((SELECT SUM(value) FROM utxo WHERE spendable_height IS NOT NULL AND spending_height IS NULL AND account_address=$1),0),
COALESCE((SELECT SUM(value) FROM utxo WHERE spending_height IS NOT NULL AND spent_height IS NULL AND account_address=$1),0)`, accountID)
	err = row.Scan(&bal.IncomingBalance, &bal.CurrentBalance, &bal.OutgoingBalance)
	if err != nil {
		return giga.AccountBalance{}, s.dbErr(err, "CalculateBalance: row.Scan")
	}
	return
}

func (s SQLiteStore) listAccountsModifiedCommon(tx Queryable, cursor int64, limit int) (ids []string, nextCursor int64, err error) {
	// MUST order by chain-seq to support the cursor API.
	nextCursor = cursor // preserve cursor in case of error.
	maxSeq := cursor
	rows_found := 0
	rows, err := tx.Query("SELECT address, chain_seq FROM account WHERE chain_seq>$1 ORDER BY chain_seq LIMIT $2", cursor, limit)
	if err != nil {
		err = s.dbErr(err, "ListAccountsModified: querying")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var seq int64
		err = rows.Scan(&id, &seq)
		if err != nil {
			err = s.dbErr(err, "ListAccountsModified: scanning row")
			return
		}
		ids = append(ids, id)
		if seq > maxSeq {
			maxSeq = seq
		}
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		err = s.dbErr(err, "ListAccountsModified: querying invoices")
		return
	}
	// Only advance nextCursor on success, up to the last seq we found.
	nextCursor = maxSeq
	return
}

type Scannable interface {
	Scan(dest ...any) error
}

// These must match the row.Scan in scanInvoice below.
const invoice_select_cols = `invoice_address, account_address, items, key_index, block_id, confirmations, created, total, paid_height, paid_event, last_incoming, last_paid,
COALESCE((SELECT SUM(value) FROM utxo WHERE added_height IS NOT NULL AND script_address=invoice.invoice_address),0) AS incoming_amount,
COALESCE((SELECT SUM(value) FROM utxo WHERE spendable_height IS NOT NULL AND script_address=invoice.invoice_address),0) AS paid_amount`

func (s SQLiteStore) scanInvoice(row Scannable, invoiceID giga.Address) (giga.Invoice, error) {
	var items_json string
	var paid_height sql.NullInt64
	var block_id sql.NullString
	var paid_event sql.NullTime
	var incoming_amount sql.NullString
	var paid_amount sql.NullString
	var last_incoming sql.NullString
	var last_paid sql.NullString
	inv := giga.Invoice{}
	err := row.Scan(&inv.ID, &inv.Account, &items_json, &inv.KeyIndex, &block_id, &inv.Confirmations, &inv.Created, &inv.Total, &paid_height, &paid_event, &last_incoming, &last_paid, &incoming_amount, &paid_amount)
	if err == sql.ErrNoRows {
		return inv, giga.NewErr(giga.NotFound, "invoice not found: %v", invoiceID)
	}
	if err != nil {
		return inv, s.dbErr(err, "ScanInvoice: row.Scan")
	}
	err = json.Unmarshal([]byte(items_json), &inv.Items)
	if err != nil {
		return inv, s.dbErr(err, "ScanInvoice: json.Unmarshal")
	}
	if paid_height.Valid {
		inv.PaidHeight = paid_height.Int64
	}
	if block_id.Valid {
		inv.BlockID = block_id.String
	}
	if paid_event.Valid {
		inv.PaidEvent = paid_event.Time
	}
	if incoming_amount.Valid {
		inv.IncomingAmount, err = decimal.NewFromString(incoming_amount.String)
		if err != nil {
			return inv, s.dbErr(err, "ScanInvoice: decimal incoming_amount")
		}
	}
	if paid_amount.Valid {
		inv.PaidAmount, err = decimal.NewFromString(paid_amount.String)
		if err != nil {
			return inv, s.dbErr(err, "ScanInvoice: decimal paid_amount")
		}
	}
	if last_incoming.Valid {
		inv.LastIncomingAmount, err = decimal.NewFromString(last_incoming.String)
		if err != nil {
			return inv, s.dbErr(err, "ScanInvoice: decimal last_incoming")
		}
	}
	if last_paid.Valid {
		inv.LastPaidAmount, err = decimal.NewFromString(last_paid.String)
		if err != nil {
			return inv, s.dbErr(err, "ScanInvoice: decimal last_paid")
		}
	}
	return inv, nil
}

var get_invoice_sql = fmt.Sprintf("SELECT %s FROM invoice WHERE invoice_address = $1", invoice_select_cols)

func (s SQLiteStore) getInvoiceCommon(tx Queryable, addr giga.Address) (giga.Invoice, error) {
	return s.scanInvoice(tx.QueryRow(get_invoice_sql, addr), addr)
}

// MUST order by key_index (or SQLite OID) to support the cursor API:
// we need a way to resume the query next time from whatever next_cursor we return,
// and the aggregate result SHOULD be stable even as the DB is modified.
var list_invoices_sql = fmt.Sprintf("SELECT %s FROM invoice WHERE account_address = $1 AND key_index >= $2 ORDER BY key_index LIMIT $3", invoice_select_cols)

func (s SQLiteStore) listInvoicesCommon(tx Queryable, account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := tx.Query(list_invoices_sql, account, cursor, limit)
	if err != nil {
		return nil, 0, s.dbErr(err, "ListInvoices: querying invoices")
	}
	defer rows.Close()
	for rows.Next() {
		inv, err := s.scanInvoice(rows, account)
		if err != nil {
			return nil, 0, err // already s.dbErr
		}
		items = append(items, inv)
		after_this := int(inv.KeyIndex) + 1
		if after_this > next_cursor {
			next_cursor = after_this // NB. starting cursor for next call
		}
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, 0, s.dbErr(err, "ListInvoices: querying invoices")
	}
	if rows_found < limit {
		// in this backend, we know there are no more rows to follow.
		next_cursor = 0 // meaning "end of query results"
	}
	return
}

// These must match the row.Scan in scanPayment below.
const payment_select_cols = "id, account_address, total, fee, created, paid_txid, paid_height, confirmed_height, on_chain_event, confirmed_event, unconfirmed_event"

func (s SQLiteStore) scanPayment(row Scannable, account giga.Address) (giga.Payment, error) {
	var paid_txid sql.NullString
	var paid_height sql.NullInt64
	var confirmed_height sql.NullInt64
	var on_chain_event sql.NullTime
	var confirmed_event sql.NullTime
	var unconfirmed_event sql.NullTime
	pay := giga.Payment{}
	err := row.Scan(&pay.ID, &pay.AccountAddress, &pay.Total, &pay.Fee, &pay.Created, &paid_txid, &paid_height, &confirmed_height, &on_chain_event, &confirmed_event, &unconfirmed_event)
	if err == sql.ErrNoRows {
		return pay, giga.NewErr(giga.NotFound, "payment not found: %v", account)
	}
	if err != nil {
		return pay, s.dbErr(err, "ScanPayment: row.Scan")
	}
	if paid_txid.Valid {
		pay.PaidTxID = paid_txid.String
	}
	if paid_height.Valid {
		pay.PaidHeight = paid_height.Int64
	}
	if confirmed_height.Valid {
		pay.ConfirmedHeight = confirmed_height.Int64
	}
	if on_chain_event.Valid {
		pay.OnChainEvent = on_chain_event.Time
	}
	if confirmed_event.Valid {
		pay.ConfirmedEvent = confirmed_event.Time
	}
	if unconfirmed_event.Valid {
		pay.UnconfirmedEvent = unconfirmed_event.Time
	}
	return pay, nil
}

var get_payment_sql = fmt.Sprintf("SELECT %s FROM payment WHERE id = $1 AND account_address = $2", payment_select_cols)

func (s SQLiteStore) getPaymentOutputs(tx Queryable, payment_id int64) (result []giga.PayTo, err error) {
	rows, err := tx.Query("SELECT pay_to, amount, deduct_fee_percent FROM output WHERE payment_id=$1 ORDER BY vout", payment_id)
	if err != nil {
		return nil, s.dbErr(err, "getPaymentOutputs: querying PayTo")
	}
	defer rows.Close()
	for rows.Next() {
		pay := giga.PayTo{}
		err := rows.Scan(&pay.PayTo, &pay.Amount, &pay.DeductFeePercent)
		if err != nil {
			return nil, s.dbErr(err, "getPaymentOutputs: scanning PayTo")
		}
		result = append(result, pay)
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, s.dbErr(err, "getPaymentOutputs: querying PayTo")
	}
	return
}

func (s SQLiteStore) getPaymentCommon(tx Queryable, account giga.Address, id int64) (giga.Payment, error) {
	pay, err := s.scanPayment(tx.QueryRow(get_payment_sql, id, account), account)
	if err == nil {
		pay.PayTo, err = s.getPaymentOutputs(tx, pay.ID)
	}
	return pay, err
}

var list_payments_sql = fmt.Sprintf("SELECT %s FROM payment WHERE account_address = $1 AND id >= $2 ORDER BY id LIMIT $3", payment_select_cols)

func (s SQLiteStore) listPaymentsCommon(tx Queryable, account giga.Address, cursor int64, limit int) (items []giga.Payment, next_cursor int64, err error) {
	rows, err := tx.Query(list_payments_sql, account, cursor, limit)
	if err != nil {
		return nil, 0, s.dbErr(err, "ListPayments: querying payments")
	}
	defer rows.Close()
	for rows.Next() {
		pay, err := s.scanPayment(rows, account)
		if err != nil {
			return nil, 0, err // already s.dbErr
		}
		items = append(items, pay)
		next_cursor = pay.ID + 1
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, 0, s.dbErr(err, "ListPayments: querying invoices")
	}
	for _, pay := range items {
		pay.PayTo, err = s.getPaymentOutputs(tx, pay.ID)
		if err != nil {
			return nil, 0, err // already s.dbErr
		}
	}
	if len(items) < limit {
		next_cursor = 0 // meaning "end of query results"
	}
	return
}

func (s SQLiteStore) getAllUnreservedUTXOsCommon(tx Queryable, account giga.Address) (result []giga.UTXO, err error) {
	// • spendable_height > 0    –– the UTXO Txn has been "confirmed" (included in CurrentBalance)
	// • spending_height IS NULL –– the UTXO has not already been spent (not yet in OutgoingBalance)
	rows_found := 0
	rows, err := tx.Query("SELECT txn_id, vout, value, script, script_type, script_address, key_index, is_internal FROM utxo WHERE account_address = $1 AND spendable_height > 0 AND spending_height IS NULL", account)
	if err != nil {
		return nil, s.dbErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	defer rows.Close()
	for rows.Next() {
		utxo := giga.UTXO{AccountID: account}
		// var value string
		err := rows.Scan(&utxo.TxID, &utxo.VOut, &utxo.Value, &utxo.ScriptHex, &utxo.ScriptType, &utxo.ScriptAddress, &utxo.KeyIndex, &utxo.IsInternal)
		if err != nil {
			return nil, s.dbErr(err, "GetAllUnreservedUTXOs: scanning UTXO row")
		}
		result = append(result, utxo)
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, s.dbErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	return
}

/****** SQLiteStoreTransaction implements giga.StoreTransaction ******/
var _ giga.StoreTransaction = &SQLiteStoreTransaction{}

type SQLiteStoreTransaction struct {
	tx       *sql.Tx
	finality bool
	store    SQLiteStore
}

func (t *SQLiteStoreTransaction) Commit() error {
	err := t.tx.Commit()
	if err != nil {
		return err
	}
	t.finality = true
	return nil
}

func (t SQLiteStoreTransaction) Rollback() error {
	if !t.finality {
		return t.tx.Rollback()
	}
	return nil
}

func (t SQLiteStoreTransaction) GetAccount(foreignID string) (giga.Account, error) {
	return t.store.getAccountCommon(t.tx, foreignID, true /*isForeignKey*/)
}

func (t SQLiteStoreTransaction) GetAccountByID(ID giga.Address) (giga.Account, error) {
	return t.store.getAccountCommon(t.tx, string(ID), false /*isForeignKey*/)
}

func (t SQLiteStoreTransaction) CalculateBalance(accountID giga.Address) (giga.AccountBalance, error) {
	return t.store.calculateBalanceCommon(t.tx, accountID)
}

func (t SQLiteStoreTransaction) ListAccountsModifiedSince(cursor int64, limit int) (ids []string, nextCursor int64, err error) {
	return t.store.listAccountsModifiedCommon(t.tx, cursor, limit)
}

func (t SQLiteStoreTransaction) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	return t.store.getInvoiceCommon(t.tx, addr)
}

func (t SQLiteStoreTransaction) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	return t.store.listInvoicesCommon(t.tx, account, cursor, limit)
}

func (t SQLiteStoreTransaction) GetPayment(account giga.Address, id int64) (giga.Payment, error) {
	return t.store.getPaymentCommon(t.tx, account, id)
}

func (t SQLiteStoreTransaction) UpdatePaymentWithTxID(paymentID int64, txID string) error {
	_, err := t.tx.Exec("UPDATE payment SET paid_txid=$1 WHERE id=$2", txID, paymentID)
	if err != nil {
		return t.store.dbErr(err, "UpdatePayment: stmt.Exec update")
	}
	return nil
}

func (t SQLiteStoreTransaction) ListPayments(account giga.Address, cursor int64, limit int) (items []giga.Payment, next_cursor int64, err error) {
	return t.store.listPaymentsCommon(t.tx, account, cursor, limit)
}

func (t SQLiteStoreTransaction) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	return t.store.getAllUnreservedUTXOsCommon(t.tx, account)
}

func (t SQLiteStoreTransaction) GetChainState() (giga.ChainState, error) {
	return t.store.getChainStateCommon(t.tx)
}

// Store an invoice
func (t SQLiteStoreTransaction) StoreInvoice(inv giga.Invoice) error {
	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		return t.store.dbErr(err, "StoreInvoice: json.Marshal items")
	}
	total := inv.CalcTotal()
	_, err = t.tx.Exec(
		"insert into invoice(invoice_address, account_address, items, total, key_index, confirmations, created) values($1,$2,$3,$4,$5,$6,$7)",
		inv.ID, inv.Account, string(items_b), total, inv.KeyIndex, inv.Confirmations, inv.Created,
	)
	if err != nil {
		return t.store.dbErr(err, "StoreInvoice: insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) CreatePayment(accountAddr giga.Address, payTo []giga.PayTo, total giga.CoinAmount, fee giga.CoinAmount) (giga.Payment, error) {
	stmt, err := t.tx.Prepare("INSERT INTO output (payment_id, vout, pay_to, amount, deduct_fee_percent) VALUES ($1,$2,$3,$4,$5)")
	if err != nil {
		return giga.Payment{}, t.store.dbErr(err, "CreatePayment: preparing insert")
	}
	defer stmt.Close()
	now := time.Now()
	row := t.tx.QueryRow(
		"INSERT INTO payment (account_address, created, total, fee) VALUES ($1,$2,$3,$4) RETURNING ID",
		accountAddr, now, total, fee)
	var id int64
	err = row.Scan(&id)
	if err != nil {
		return giga.Payment{}, t.store.dbErr(err, "createPayment: insert")
	}
	for vout, pt := range payTo {
		_, err = stmt.Exec(id, vout, pt.PayTo, pt.Amount, pt.DeductFeePercent)
		if err != nil {
			return giga.Payment{}, t.store.dbErr(err, "CreatePayment: executing insert")
		}
	}
	return giga.Payment{
		ID:             id,
		AccountAddress: accountAddr,
		PayTo:          payTo,
		Total:          total,
		Fee:            fee,
		Created:        now,
	}, nil
}

func (t SQLiteStoreTransaction) CreateAccount(acc giga.Account) error {
	_, err := t.tx.Exec(
		"insert into account(foreign_id,address,privkey,next_int_key,next_ext_key,next_pool_int,next_pool_ext,payout_address,payout_threshold,payout_frequency) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
		acc.ForeignID, acc.Address, acc.Privkey, // only in createAccount.
		acc.NextInternalKey, acc.NextExternalKey, // common (see updateAccount) ...
		acc.NextPoolInternal, acc.NextPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency)
	if err != nil {
		return t.store.dbErr(err, "createAccount: executing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) UpdateAccount(acc giga.Account) error {
	sql := "UPDATE account SET next_int_key=MAX(next_int_key,$1), next_ext_key=MAX(next_ext_key,$2), next_pool_int=MAX(next_pool_int,$3), next_pool_ext=MAX(next_pool_ext,$4), payout_address=$5, payout_threshold=$6, payout_frequency=$7 WHERE foreign_id=$8"
	if t.store.isPostgres {
		sql = "UPDATE account SET next_int_key=GREATEST(next_int_key,$1), next_ext_key=GREATEST(next_ext_key,$2), next_pool_int=GREATEST(next_pool_int,$3), next_pool_ext=GREATEST(next_pool_ext,$4), payout_address=$5, payout_threshold=$6, payout_frequency=$7 WHERE foreign_id=$8"
	}
	res, err := t.tx.Exec(sql,
		acc.NextInternalKey, acc.NextExternalKey, // common (see createAccount) ...
		acc.NextPoolInternal, acc.NextPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency,
		acc.ForeignID) // the Key (not updated)
	return t.checkRowsAffected(res, err, "account", acc.ForeignID)
}

func (t SQLiteStoreTransaction) UpdateAccountBalance(accountID giga.Address, bal giga.AccountBalance) error {
	res, err := t.tx.Exec(
		"UPDATE account SET current_balance=$1, incoming_balance=$2, outgoing_balance=$3 WHERE address=$4",
		bal.CurrentBalance, bal.IncomingBalance, bal.OutgoingBalance, accountID)
	return t.checkRowsAffected(res, err, "account", string(accountID))
}

func (t SQLiteStoreTransaction) checkRowsAffected(res sql.Result, err error, what string, id string) error {
	if err != nil {
		return t.store.dbErr(err, fmt.Sprintf("Executing update: %s: %s", what, id))
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return t.store.dbErr(err, fmt.Sprintf("Checking RowsAffected: %s: %s", what, id))
	}
	if num_rows < 1 {
		// MUST detect this error to fulfil the API contract.
		return giga.NewErr(giga.NotFound, fmt.Sprintf("%s not found: %s", what, id))
	}
	return nil
}

func (t SQLiteStoreTransaction) StoreAddresses(accountID giga.Address, addresses []giga.Address, firstAddress uint32, isInternal bool) error {
	// Associate a list of addresses with an accountID in the account_address table.
	stmt, err := t.tx.Prepare("INSERT INTO account_address (address,key_index,is_internal,account_address) VALUES ($1,$2,$3,$4)")
	if err != nil {
		return t.store.dbErr(err, "StoreAddresses: preparing insert")
	}
	defer stmt.Close()
	firstKey := firstAddress
	for n, addr := range addresses {
		_, err = stmt.Exec(addr, firstKey+uint32(n), isInternal, accountID)
		if err != nil {
			return t.store.dbErr(err, "StoreAddresses: executing insert")
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) FindAccountForAddress(address giga.Address) (giga.Address, uint32, bool, error) {
	row := t.tx.QueryRow("SELECT account_address, key_index, is_internal FROM account_address WHERE address = $1", address)
	var accountID giga.Address
	var keyIndex uint32
	var isInternal bool
	err := row.Scan(&accountID, &keyIndex, &isInternal)
	if err == sql.ErrNoRows {
		return "", 0, false, giga.NewErr(giga.NotFound, "no matching account for address: %s", address)
	}
	if err != nil {
		return "", 0, false, t.store.dbErr(err, "FindAccountForAddress: error scanning row")
	}
	return accountID, keyIndex, isInternal, nil
}

func (t SQLiteStoreTransaction) UpdateChainState(state giga.ChainState, writeRoot bool) error {
	var res sql.Result
	var err error
	if writeRoot {
		res, err = t.tx.Exec("UPDATE chainstate SET best_hash=$1, best_height=$2, root_hash=$3, first_height=$4, next_seq=$5", state.BestBlockHash, state.BestBlockHeight, state.RootHash, state.FirstHeight, state.NextSeq)
	} else {
		res, err = t.tx.Exec("UPDATE chainstate SET best_hash=$1, best_height=$2, next_seq=$3", state.BestBlockHash, state.BestBlockHeight, state.NextSeq)
	}
	if err != nil {
		return t.store.dbErr(err, "UpdateChainState: executing update")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return t.store.dbErr(err, "UpdateChainState: res.RowsAffected")
	}
	if num_rows < 1 {
		// this is the first call to UpdateChainState: insert the row.
		_, err = t.tx.Exec("INSERT INTO chainstate (best_hash,best_height,root_hash,first_height,next_seq) VALUES ($1,$2,$3,$4,$5)", state.BestBlockHash, state.BestBlockHeight, state.RootHash, state.FirstHeight, state.NextSeq)
		if err != nil {
			return t.store.dbErr(err, "UpdateChainState: executing insert")
		}
	}
	return nil
}

const create_utxo_sqlite = "INSERT INTO utxo (txn_id, vout, value, script, script_type, script_address, account_address, key_index, is_internal, added_height) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO UPDATE SET value=$3, script=$4, script_type=$5, script_address=$6, account_address=$7, key_index=$8, is_internal=$9, added_height=$10 WHERE txn_id=$1 AND vout=$2"
const create_utxo_psql = "INSERT INTO utxo (txn_id, vout, value, script, script_type, script_address, account_address, key_index, is_internal, added_height) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT ON CONSTRAINT utxo_pkey DO UPDATE SET value=$3, script=$4, script_type=$5, script_address=$6, account_address=$7, key_index=$8, is_internal=$9, added_height=$10"

func (t SQLiteStoreTransaction) CreateUTXO(utxo giga.UTXO) error {
	// Create a new Unspent Transaction Output in the database.
	// Updates Account 'incoming' to indicate unconfirmed funds.
	// For psql: "ON CONFLICT ON CONSTRAINT utxo_pkey DO UPDATE ..."
	// Remove in the end "WHERE txn_id=$1 AND vout=$2"
	// psql: " ..."
	sql := create_utxo_sqlite
	if t.store.isPostgres {
		sql = create_utxo_psql
	}
	_, err := t.tx.Exec(
		sql, utxo.TxID, utxo.VOut, utxo.Value, utxo.ScriptHex, utxo.ScriptType, utxo.ScriptAddress,
		utxo.AccountID, utxo.KeyIndex, utxo.IsInternal, utxo.BlockHeight,
	)
	if err != nil {
		return t.store.dbErr(err, "CreateUTXO: preparing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) MarkUTXOSpent(txID string, vOut int, blockHeight int64, spendTxID string) (id string, scriptAddress giga.Address, err error) {
	row := t.tx.QueryRow("UPDATE utxo SET spending_height=$1, spend_txid=$2 WHERE txn_id=$3 AND vout=$4 RETURNING account_address, script_address", blockHeight, spendTxID, txID, vOut)
	if err != nil {
		return "", "", t.store.dbErr(err, "MarkUTXOSpent: executing update")
	}
	err = row.Scan(&id, &scriptAddress)
	if err == sql.ErrNoRows {
		return "", "", nil // commonly called for UTXOs we don't have in the DB.
	}
	if err != nil {
		return "", "", t.store.dbErr(err, "MarkUTXOSpent: scanning row")
	}
	return
}

// Mark payments paid that match any of the txIDs (storing the given block-height)
// Returns the IDs of the Accounts that own any affected payments (can have duplicates)
func (t SQLiteStoreTransaction) MarkPaymentsOnChain(txIDs []string, blockHeight int64) (accounts []string, err error) {
	stmt, err := t.tx.Prepare("UPDATE payment SET paid_height=$1 WHERE paid_txid=$2 RETURNING account_address")
	if err != nil {
		return nil, t.store.dbErr(err, "MarkPaymentsOnChain: preparing update")
	}
	for id := range txIDs {
		rows, err := stmt.Query(blockHeight, id)
		if accounts, err = collectArrayIDs(rows, err, accounts); err != nil {
			return nil, t.store.dbErr(err, "MarkPaymentsOnChain")
		}
	}
	return
}

func (t SQLiteStoreTransaction) ConfirmPayments(confirmations int, blockHeight int64) (affectedAccounts []string, err error) {
	// note: there is an index on (paid_height) for this query.
	rows, err := t.tx.Query(
		"UPDATE payment SET confirmed_height = paid_height + $1 WHERE paid_height + $1 <= $2 AND confirmed_height IS NULL RETURNING account_address",
		confirmations, blockHeight,
	)
	if affectedAccounts, err = collectArrayIDs(rows, err, affectedAccounts); err != nil {
		return nil, t.store.dbErr(err, "ConfirmPayments: updating payments")
	}
	return
}

// There is an index on (added_height, spendable_height) for this query.
// This uses #confirmations from the invoice being paid, or the configured #confirmations.
// This MUST be a LEFT OUTER join (script_address may not match any invoice)
var confirm_spendable_sql = `UPDATE utxo SET spendable_height = added_height +
	COALESCE((SELECT confirmations FROM invoice WHERE invoice_address = utxo.script_address),$1) WHERE added_height +
	COALESCE((SELECT confirmations FROM invoice WHERE invoice_address = utxo.script_address),$2) <= $3 AND spendable_height IS NULL RETURNING account_address`
var confirm_spent_sql = "UPDATE utxo SET spent_height = spending_height + $1 WHERE spending_height + $2 <= $3 AND spent_height IS NULL RETURNING account_address"

func (t SQLiteStoreTransaction) ConfirmUTXOs(confirmations int, blockHeight int64) (affectedAccounts []string, err error) {
	// confirm_spendable
	rows, err := t.tx.Query(confirm_spendable_sql, confirmations, confirmations, blockHeight)
	if affectedAccounts, err = collectArrayIDs(rows, err, affectedAccounts); err != nil {
		return nil, t.store.dbErr(err, "ConfirmUTXOs: confirming spendable")
	}
	// confirm_spent
	rows, err = t.tx.Query(confirm_spent_sql, confirmations, confirmations, blockHeight)
	if affectedAccounts, err = collectArrayIDs(rows, err, affectedAccounts); err != nil {
		return nil, t.store.dbErr(err, "ConfirmUTXOs: confirming spent")
	}
	return
}

const incoming_amount_sql = "UPDATE invoice SET last_incoming=COALESCE((SELECT SUM(value) FROM utxo WHERE added_height IS NOT NULL AND script_address=invoice.invoice_address),0) WHERE invoice_address=$1"
const paid_amount_sql = "UPDATE invoice SET last_paid=COALESCE((SELECT SUM(value) FROM utxo WHERE spendable_height IS NOT NULL AND script_address=invoice.invoice_address),0) WHERE invoice_address=$1"

func (t SQLiteStoreTransaction) MarkInvoiceEventSent(invoiceID giga.Address, event giga.EVENT_INV) error {
	sql := ""
	switch event {
	case giga.INV_PART_PAYMENT_DETECTED, giga.INV_TOTAL_PAYMENT_DETECTED, giga.INV_OVER_PAYMENT_DETECTED:
		// set LastIncomingAmount = IncomingAmount
		sql = incoming_amount_sql // "UPDATE invoice SET last_incoming=incoming_amount WHERE invoice_address=$1"
	case giga.INV_OVER_PAYMENT_CONFIRMED:
		// set LastPaidAmount = PaidAmount
		sql = paid_amount_sql // "UPDATE invoice SET last_paid=paid_amount WHERE invoice_address=$1"
	case giga.INV_TOTAL_PAYMENT_CONFIRMED:
		// set paid_event = NOW
		sql = "UPDATE invoice SET paid_event=CURRENT_TIMESTAMP WHERE invoice_address=$1"
	case giga.INV_PAYMENT_UNCONFIRMED:
		// set PaidEvent = NULL
		sql = "UPDATE invoice SET paid_event=NULL WHERE invoice_address=$1"
	default:
		return giga.NewErr(giga.BadRequest, "unsupported event")
	}
	_, err := t.tx.Exec(sql, invoiceID)
	if err != nil {
		return t.store.dbErr(err, "MarkInvoiceEventSent: UPDATE")
	}
	return nil
}

// Prepare query for MarkInvoicesPaid.
// Summing all UTXOs that payTo the Invoice Address that have been confirmed (spendable_height is non-null)
var sum_utxos_for_invoice = "SELECT SUM(value) FROM utxo WHERE script_address=i.invoice_address AND spendable_height IS NOT NULL"
var unpaid_invoices_above_total = fmt.Sprintf("SELECT invoice_address FROM invoice i WHERE paid_height IS NULL AND (%s) >= total", sum_utxos_for_invoice)
var mark_invoices_paid = fmt.Sprintf("UPDATE invoice SET paid_height=$1, block_id=$2 WHERE invoice_address IN (%s) RETURNING account_address", unpaid_invoices_above_total)

// Mark all invoices paid that have corresponding confirmed UTXOs [via ConfirmUTXOs]
// that sum up to the invoice total, storing the given block-height. Returns the IDs
// of the Accounts that own any affected invoices (can return duplicates)
func (t SQLiteStoreTransaction) MarkInvoicesPaid(blockHeight int64, blockID string) (accounts []string, err error) {
	rows, err := t.tx.Query(mark_invoices_paid, blockHeight, blockID)
	if accounts, err = collectArrayIDs(rows, err, accounts); err != nil {
		return nil, t.store.dbErr(err, "MarkInvoicesPaid")
	}
	return
}

func collectArrayIDs(rows *sql.Rows, err error, ids []string) ([]string, error) {
	if err != nil {
		return ids, err
	}
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			rows.Close()
			return ids, err
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

func collectIDs(rows *sql.Rows, dbErr error, accounts map[string]int64, seq int64) (int64, error) {
	if dbErr != nil {
		return seq, dbErr
	}
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			rows.Close()
			return seq, err
		}
		if _, present := accounts[id]; !present {
			accounts[id] = seq
			seq += 1
		}
	}
	return seq, rows.Err()
}

func (t SQLiteStoreTransaction) RevertChangesAboveHeight(maxValidHeight int64, seq int64) (int64, error) {
	// UTXOs.
	// The presence of a height in added_height, spendable_height, spending_height, spent_height
	// indicates that the UTXO is in the process of being added, or has been added (confirmed);
	// is reserved for spending, or has been spent (confirmed)
	// When we undo one of these, we always undo the stages that happen later as well.
	accounts := make(map[string]int64)
	rows, err := t.tx.Query("UPDATE utxo SET added_height=NULL,spendable_height=NULL,spending_height=NULL,spent_height=NULL WHERE added_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 1")
	}
	rows, err = t.tx.Query("UPDATE utxo SET spendable_height=NULL,spending_height=NULL,spent_height=NULL WHERE spendable_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 2")
	}
	rows, err = t.tx.Query("UPDATE utxo SET spending_height=NULL,spent_height=NULL WHERE spending_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 3")
	}
	rows, err = t.tx.Query("UPDATE utxo SET spent_height=NULL WHERE spent_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 4")
	}
	// Invoices.
	// Presence of paid_height means MarkInvoicesPaid has seen sum(utxos) > total where
	// the UTXOs have been marked as confirmed (i.e. N confirmations where N comes from the invoice!)
	rows, err = t.tx.Query("UPDATE invoice SET paid_height=NULL,block_id=NULL WHERE paid_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: invoice update")
	}
	// Payments.
	// Presence of paid_height means MarkPaymentsOnChain has seen the payment in a block.
	// If we undo this, we also undo confirmed_height (which happens later)
	rows, err = t.tx.Query("UPDATE payment SET paid_height=NULL,confirmed_height=NULL WHERE paid_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: payment update 1")
	}
	// Presence of confirmed_height means ConfirmPayments has seen N confirmations.
	rows, err = t.tx.Query("UPDATE payment SET confirmed_height=NULL WHERE confirmed_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: payment update 2")
	}
	return seq, t.IncChainSeqForAccounts(accounts)
}

func (t SQLiteStoreTransaction) IncChainSeqForAccounts(accounts map[string]int64) error {
	// Increment the chain-sequence-number for multiple accounts.
	// Use this after modifying accounts' blockchain-derived state (UTXOs, TXNs)
	stmt, err := t.tx.Prepare("UPDATE account SET chain_seq=$1 WHERE address=$2")
	if err != nil {
		return t.store.dbErr(err, "IncAccountChainSeq: prepare")
	}
	defer stmt.Close()
	for key, seq := range accounts {
		_, err := stmt.Exec(seq, key)
		if err != nil {
			return t.store.dbErr(err, "IncAccountChainSeq: update "+key)
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) SetServiceCursor(name string, cursor int64) error {
	res, err := t.tx.Exec("UPDATE services SET cursor=$1 WHERE name=$2", cursor, name)
	if err != nil {
		return t.store.dbErr(err, "SetServiceCursor: UPDATE")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return t.store.dbErr(err, "SetServiceCursor: RowsAffected")
	}
	if num_rows < 1 {
		// this is the first call to SetServiceCursor for this service: insert the row.
		_, err = t.tx.Exec("INSERT INTO services (name,cursor) VALUES ($1,$2)", name, cursor)
		if err != nil {
			return t.store.dbErr(err, "SetServiceCursor: INSERT")
		}
	}
	return nil
}

func (s SQLiteStore) dbErr(err error, where string) error {
	if s.isPostgres {
		if pqErr, isPq := err.(*pq.Error); isPq {
			name := pqErr.Code.Name()
			if name == "unique_violation" {
				// MUST detect 'AlreadyExists' to fulfil the API contract!
				return giga.NewErr(giga.AlreadyExists, "PostgresStore error: %s: %v", where, err)
			}
			if name == "serialization_failure" || name == "transaction_integrity_constraint_violation" {
				// Transaction rollback due to serialization conflict.
				// Transient database conflict: the caller should retry.
				return giga.NewErr(giga.DBConflict, "PostgresStore error: %s: %v", where, err)
			}
		}
		return giga.NewErr(giga.NotAvailable, "PostgresStore error: %s: %v", where, err)
	} else {
		if sqErr, isSq := err.(sqlite3.Error); isSq {
			if sqErr.Code == sqlite3.ErrConstraint {
				// MUST detect 'AlreadyExists' to fulfil the API contract!
				// Constraint violation, e.g. a duplicate key.
				return giga.NewErr(giga.AlreadyExists, "SQLiteStore error: %s: %v", where, err)
			}
			if sqErr.Code == sqlite3.ErrBusy || sqErr.Code == sqlite3.ErrLocked {
				// SQLite has a single-writer policy, even in WAL (write-ahead) mode.
				// SQLite will return BUSY if the database is locked by another connection.
				// We treat this as a transient database conflict, and the caller should retry.
				return giga.NewErr(giga.DBConflict, "SQLiteStore error: %s: %v", where, err)
			}
		}
		return giga.NewErr(giga.NotAvailable, "SQLiteStore error: %s: %v", where, err)
	}
}
