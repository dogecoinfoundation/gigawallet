package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/lib/pq"

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
	payout_threshold NUMERIC NOT NULL,
	payout_frequency TEXT NOT NULL,
	current_balance NUMERIC NOT NULL DEFAULT 0,
	incoming_balance NUMERIC NOT NULL DEFAULT 0,
	outgoing_balance NUMERIC NOT NULL DEFAULT 0,
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
	txn_id TEXT NOT NULL,
	vendor TEXT NOT NULL,
	items TEXT NOT NULL,
	total NUMERIC NOT NULL,
	key_index INTEGER NOT NULL,
	block_id TEXT NOT NULL,
	confirmations INTEGER NOT NULL,
	created DATETIME NOT NULL,
	paid_height INTEGER,
	notified DATETIME
);
CREATE INDEX IF NOT EXISTS invoice_account_i ON invoice (account_address);

CREATE TABLE IF NOT EXISTS payment (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_address TEXT NOT NULL,
	pay_to TEXT NOT NULL,
	amount INTEGER NOT NULL,
	created DATETIME NOT NULL,
	paid_txid TEXT,
	paid_height INTEGER,
	confirmed_height INTEGER,
	notified DATETIME
);

CREATE INDEX IF NOT EXISTS payment_account_i ON payment (account_address);
CREATE INDEX IF NOT EXISTS payment_txid_i ON payment (paid_txid);
CREATE INDEX IF NOT EXISTS payment_paid_height_i ON payment (paid_height);

CREATE TABLE IF NOT EXISTS utxo (
	txn_id TEXT NOT NULL,
	vout INTEGER NOT NULL,
	value TEXT NOT NULL,
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
)
`

// Prepare query for MarkInvoicesPaid:
var sum_utxos_for_invoice = "SELECT SUM(value) FROM utxo WHERE script_address=i.invoice_address AND spendable_height IS NOT NULL"
var invoices_above_total = fmt.Sprintf("SELECT invoice_address FROM invoice i WHERE (%s) >= total", sum_utxos_for_invoice)
var mark_invoices_paid = fmt.Sprintf("UPDATE invoice SET paid_height=$1 WHERE invoice_address IN (%s) RETURNING account_address", invoices_above_total)

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
	if backend == "sqlite3" {
		// limit concurrent access until we figure out a way to start transactions
		// with the BEGIN CONCURRENT statement in Go.
		db.SetMaxOpenConns(1)
		// WAL mode provides more concurrency, although not necessary with above.
		// _, err = db.Exec("PRAGMA journal_mode=WAL")
		// if err != nil {
		// 	return SQLiteStore{}, store.dbErr(err, "creating database schema")
		// }
	}
	// init tables / indexes
	_, err = db.Exec(SETUP_SQL)
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
	row := s.db.QueryRow("SELECT best_hash, best_height, root_hash, first_height, next_seq FROM chainstate")
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

func (s SQLiteStore) getInvoiceCommon(tx Queryable, addr giga.Address) (giga.Invoice, error) {
	row := tx.QueryRow("SELECT invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations, created FROM invoice WHERE invoice_address = $1", addr)
	var id giga.Address
	var account giga.Address
	var tx_id string
	var vendor string
	var items_json string
	var key_index uint32
	var block_id string
	var confirmations int32
	var created time.Time
	err := row.Scan(&id, &account, &tx_id, &vendor, &items_json, &key_index, &block_id, &confirmations, &created)
	if err == sql.ErrNoRows {
		return giga.Invoice{}, giga.NewErr(giga.NotFound, "invoice not found: %v", addr)
	}
	if err != nil {
		return giga.Invoice{}, s.dbErr(err, "GetInvoice: row.Scan")
	}
	var items []giga.Item
	err = json.Unmarshal([]byte(items_json), &items)
	if err != nil {
		return giga.Invoice{}, s.dbErr(err, "GetInvoice: json.Unmarshal")
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
		Created:       created,
	}, nil
}

func (s SQLiteStore) listInvoicesCommon(tx Queryable, account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// MUST order by key_index (or sqlite OID) to support the cursor API:
	// we need a way to resume the query next time from whatever next_cursor we return,
	// and the aggregate result SHOULD be stable even as the DB is modified.
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := tx.Query("SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = $1 AND key_index >= $2 ORDER BY key_index LIMIT $3", account, cursor, limit)
	if err != nil {
		return nil, 0, s.dbErr(err, "ListInvoices: querying invoices")
	}
	defer rows.Close()
	for rows.Next() {
		inv := giga.Invoice{Account: account}
		var items_json string
		err := rows.Scan(&inv.ID, &inv.TXID, &inv.Vendor, &items_json, &inv.KeyIndex, &inv.BlockID, &inv.Confirmations)
		if err != nil {
			return nil, 0, s.dbErr(err, "ListInvoices: scanning invoice row")
		}
		err = json.Unmarshal([]byte(items_json), &inv.Items)
		if err != nil {
			return nil, 0, s.dbErr(err, "ListInvoices: unmarshalling json")
		}
		items = append(items, inv)
		after_this := int(inv.KeyIndex) + 1 // XXX assumes non-hardened HD Key! (from uint32)
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

func (s SQLiteStore) getPaymentCommon(tx Queryable, account giga.Address, id int64) (giga.Payment, error) {
	row := tx.QueryRow("SELECT id, account_address, pay_to, amount, created, paid_txid, paid_height, notify_height FROM payment WHERE id = $1 AND account_address = $2", id, account)
	p := giga.Payment{}
	err := row.Scan(&p.ID, &p.AccountAddress, &p.PayTo, &p.Amount, &p.Created, &p.PaidTxID, &p.PaidHeight, &p.NotifyHeight)
	if err == sql.ErrNoRows {
		return p, giga.NewErr(giga.NotFound, "payment not found: %v", id)
	}
	if err != nil {
		return p, s.dbErr(err, "GetPayment: row.Scan")
	}
	return p, nil
}

func (s SQLiteStore) listPaymentsCommon(tx Queryable, account giga.Address, cursor int64, limit int) (items []giga.Payment, next_cursor int64, err error) {
	rows, err := tx.Query("SELECT id, account_address, pay_to, amount, created, paid_txid, paid_height, notify_height FROM payment WHERE account_address = $1 AND id >= $2 ORDER BY id LIMIT $3", account, cursor, limit)
	if err != nil {
		return nil, 0, s.dbErr(err, "ListPayments: querying payments")
	}
	defer rows.Close()
	for rows.Next() {
		p := giga.Payment{}
		err := rows.Scan(&p.ID, &p.AccountAddress, &p.PayTo, &p.Amount, &p.Created, &p.PaidTxID, &p.PaidHeight, &p.NotifyHeight)
		if err != nil {
			return nil, 0, s.dbErr(err, "ListPayments: scanning payment row")
		}
		items = append(items, p)
		next_cursor = p.ID + 1
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, 0, s.dbErr(err, "ListPayments: querying invoices")
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

func (t SQLiteStoreTransaction) GetAccountByID(ID string) (giga.Account, error) {
	return t.store.getAccountCommon(t.tx, ID, false /*isForeignKey*/)
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

// Store an invoice
func (t SQLiteStoreTransaction) StoreInvoice(inv giga.Invoice) error {
	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		return t.store.dbErr(err, "createInvoice: json.Marshal items")
	}
	total := inv.CalcTotal()
	_, err = t.tx.Exec(
		"insert into invoice(invoice_address, account_address, txn_id, vendor, items, total, key_index, block_id, confirmations, created) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
		inv.ID, inv.Account, inv.TXID, inv.Vendor, string(items_b), total, inv.KeyIndex, inv.BlockID, inv.Confirmations, inv.Created,
	)
	if err != nil {
		return t.store.dbErr(err, "createInvoice: insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) CreatePayment(accountAddr giga.Address, amount giga.CoinAmount, payTo giga.Address) (giga.Payment, error) {
	now := time.Now()
	row := t.tx.QueryRow(
		"insert into payment(account_address, pay_to, amount, created) values($1,$2,$3,$4) returning id",
		accountAddr, payTo, amount, now)
	var id int64
	err := row.Scan(&id)
	if err != nil {
		return giga.Payment{}, t.store.dbErr(err, "createPayment: insert")
	}
	return giga.Payment{
		ID:             id,
		AccountAddress: accountAddr,
		PayTo:          payTo,
		Amount:         amount,
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
	res, err := t.tx.Exec(
		"update account set next_int_key=MAX(next_int_key,$1), next_ext_key=MAX(next_ext_key,$2), next_pool_int=MAX(next_pool_int,$3), next_pool_ext=MAX(next_pool_ext,$4), payout_address=$5, payout_threshold=$6, payout_frequency=$7 where foreign_id=$8",
		acc.NextInternalKey, acc.NextExternalKey, // common (see createAccount) ...
		acc.NextPoolInternal, acc.NextPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency,
		acc.ForeignID) // the Key (not updated)
	return t.checkRowsAffected(res, err, "account", acc.ForeignID)
}

func (t SQLiteStoreTransaction) UpdateAccountBalance(accountID giga.Address, bal giga.AccountBalance) error {
	res, err := t.tx.Exec(
		"update account set current_balance=$1, incoming_balance=$2, outgoing_balance=$3 where address=$4",
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
	row := t.tx.QueryRow("SELECT account_address,key_index,is_internal FROM account_address WHERE address = $1", address)
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

func (t SQLiteStoreTransaction) CreateUTXO(utxo giga.UTXO) error {
	// Create a new Unspent Transaction Output in the database.
	// Updates Account 'incoming' to indicate unconfirmed funds.
	// psql: "ON CONFLICT ON CONSTRAINT utxo_pkey DO UPDATE ..."
	_, err := t.tx.Exec(
		"INSERT INTO utxo (txn_id, vout, value, script, script_type, script_address, account_address, key_index, is_internal, added_height) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT DO UPDATE SET value=$3, script=$4, script_type=$5, script_address=$6, account_address=$7, key_index=$8, is_internal=$9, added_height=$10 WHERE txn_id=$1 AND vout=$2",
		utxo.TxID, utxo.VOut, utxo.Value, utxo.ScriptHex, utxo.ScriptType, utxo.ScriptAddress, utxo.AccountID, utxo.KeyIndex, utxo.IsInternal, utxo.BlockHeight,
	)
	if err != nil {
		return t.store.dbErr(err, "CreateUTXO: preparing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) MarkUTXOSpent(txID string, vOut int, blockHeight int64, spendTxID string) (id string, scriptAddress giga.Address, err error) {
	row := t.tx.QueryRow("UPDATE utxo SET spending_height=$3, spend_txid=$4 WHERE txn_id=$1 AND vout=$2 RETURNING account_address, script_address", txID, vOut, blockHeight, spendTxID)
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
	stmt, err := t.tx.Prepare("UPDATE payment SET paid_height=$2 WHERE paid_txid=$1 RETURNING account_address")
	if err != nil {
		return nil, t.store.dbErr(err, "MarkPaymentsOnChain: preparing update")
	}
	for id := range txIDs {
		rows, err := stmt.Query(id, blockHeight)
		if err != nil {
			return nil, t.store.dbErr(err, "MarkPaymentsOnChain: executing update")
		}
		for rows.Next() {
			var account string
			err := rows.Scan(&account)
			if err != nil {
				rows.Close()
				return nil, t.store.dbErr(err, "MarkPaymentsOnChain: scanning row")
			}
			accounts = append(accounts, account)
		}
		rows.Close()
	}
	return
}

func (t SQLiteStoreTransaction) ConfirmPayments(confirmations int, blockHeight int64) (affectedAccounts []string, err error) {
	confirmHeight := blockHeight - int64(confirmations) // from config.
	// note: there is an index on (paid_height) for this query.
	rows, err := t.tx.Query(
		"UPDATE payment SET confirmed_height=$1 WHERE paid_height<=$2 AND confirmed_height IS NULL RETURNING account_address",
		blockHeight, confirmHeight,
	)
	if err != nil {
		return nil, t.store.dbErr(err, "ConfirmPayments: updating payments")
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, t.store.dbErr(err, "ConfirmUTXOs: scanning row")
		}
		if id != "" {
			affectedAccounts = append(affectedAccounts, id)
		}
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, t.store.dbErr(err, "ConfirmUTXOs: scanning rows")
	}
	return
}

func (t SQLiteStoreTransaction) ConfirmUTXOs(confirmations int, blockHeight int64) (affectedAccounts []string, err error) {
	confirmedHeight := blockHeight - int64(confirmations) // from config.
	// note: there is an index on (added_height, spendable_height) for this query.
	// note: this uses num-confirmations from the invoice being paid, if there is one.
	// note: this MUST be a LEFT OUTER join (script_address may not match any invoice)
	rows, err := t.tx.Query(`
		UPDATE utxo SET spendable_height=$1
		WHERE added_height <= COALESCE((SELECT $1 - confirmations from invoice WHERE invoice_address = utxo.script_address), $2)
		AND spendable_height IS NULL
		RETURNING account_address
	`, blockHeight, confirmedHeight)
	if err != nil {
		return nil, t.store.dbErr(err, "ConfirmUTXOs: updating utxos")
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, t.store.dbErr(err, "ConfirmUTXOs: scanning row")
		}
		if id != "" {
			affectedAccounts = append(affectedAccounts, id)
		}
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, t.store.dbErr(err, "ConfirmUTXOs: scanning rows")
	}
	return
}

// Mark all invoices paid that have corresponding confirmed UTXOs [via ConfirmUTXOs]
// that sum up to the invoice total, storing the given block-height. Returns the IDs
// of the Accounts that own any affected invoices (can return duplicates)
func (t SQLiteStoreTransaction) MarkInvoicesPaid(blockHeight int64) (accounts []string, err error) {
	rows, err := t.tx.Query(mark_invoices_paid, blockHeight)
	if err != nil {
		return nil, t.store.dbErr(err, "MarkInvoicesPaid: preparing update")
	}
	defer rows.Close()
	for rows.Next() {
		var account string
		err := rows.Scan(&account)
		if err != nil {
			return nil, t.store.dbErr(err, "MarkInvoicesPaid: scanning row")
		}
		accounts = append(accounts, account)
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, t.store.dbErr(err, "MarkInvoicesPaid: scanning rows")
	}
	return
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
	// The presence of a height in added_height, spendable_height, spending_height, spent_height
	// indicates that the UTXO is in the process of being added, or has been added (confirmed); is
	// reserved for spending, or has been spent (confirmed)
	// When we undo one of these, we always undo the stages that happen later as well.
	accounts := make(map[string]int64)
	rows1, err := t.tx.Query("UPDATE utxo SET added_height=NULL,spendable_height=NULL,spending_height=NULL,spent_height=NULL WHERE added_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows1, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 1")
	}
	rows2, err := t.tx.Query("UPDATE utxo SET spendable_height=NULL,spending_height=NULL,spent_height=NULL WHERE spendable_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows2, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 2")
	}
	rows3, err := t.tx.Query("UPDATE utxo SET spending_height=NULL,spent_height=NULL WHERE spending_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows3, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 3")
	}
	rows4, err := t.tx.Query("UPDATE utxo SET spent_height=NULL WHERE spent_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows4, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: utxo update 4")
	}
	// Presence of paid_height means MarkPaymentsOnChain has seen the payment on-chain.
	// If we undo this, we also undo confirmed_height (which happens later)
	rows5, err := t.tx.Query("UPDATE payment SET paid_height=NULL,confirmed_height=NULL WHERE paid_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows5, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: executing update 5")
	}
	// Presence of confirmed_height means ConfirmPayments has marked the payment confirmed.
	rows6, err := t.tx.Query("UPDATE payment SET confirmed_height=NULL WHERE confirmed_height>$1 RETURNING account_address", maxValidHeight)
	if seq, err = collectIDs(rows6, err, accounts, seq); err != nil {
		return seq, t.store.dbErr(err, "RevertUTXOsAboveHeight: executing update 6")
	}
	return seq, t.IncChainSeqForAccounts(accounts)
}

func (t SQLiteStoreTransaction) IncChainSeqForAccounts(accounts map[string]int64) error {
	// Increment the chain-sequence-number for multiple accounts.
	// Use this after modifying accounts' blockchain-derived state (UTXOs, TXNs)
	stmt, err := t.tx.Prepare("UPDATE account SET chain_seq=$2 WHERE address=$1")
	if err != nil {
		return t.store.dbErr(err, "IncAccountChainSeq: prepare")
	}
	defer stmt.Close()
	for key, seq := range accounts {
		_, err := stmt.Exec(key, seq)
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
