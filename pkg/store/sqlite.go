package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/shopspring/decimal"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const MINIMUM_TIMEOUT = 5 * time.Second

const SETUP_SQL string = `
CREATE TABLE IF NOT EXISTS account (
	address TEXT NOT NULL PRIMARY KEY,
	foreign_id TEXT NOT NULL UNIQUE,
	privkey TEXT NOT NULL,
	next_int_key INTEGER NOT NULL,
	next_ext_key INTEGER NOT NULL,
	max_pool_int INTEGER NOT NULL,
	max_pool_ext INTEGER NOT NULL,
	payout_address TEXT NOT NULL,
	payout_threshold TEXT NOT NULL,
	payout_frequency TEXT NOT NULL
);

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
	key_index INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS invoice_account_i ON invoice (account_address);

CREATE TABLE IF NOT EXISTS txn (
	txn_id TEXT NOT NULL PRIMARY KEY,
	account_address TEXT NOT NULL,
	invoice_address TEXT,
	on_chain_height INTEGER,
	verified_height INTEGER,
	send_verified BOOLEAN NOT NULL,
	send_rollback BOOLEAN NOT NULL
);
CREATE INDEX IF NOT EXISTS txn_account_i ON txn (account_address);

CREATE TABLE IF NOT EXISTS utxo (
	txn_id TEXT NOT NULL,
	vout INTEGER NOT NULL,
	account_address TEXT NOT NULL,
	value TEXT NOT NULL,
	script_type TEXT NOT NULL,
	script_address TEXT NOT NULL,
	key_index INTEGER NOT NULL,
	is_internal BOOLEAN NOT NULL,
	adding_height INTEGER,
	available_height INTEGER,
	spending_height INTEGER,
	spent_height INTEGER,
	PRIMARY KEY (txn_id, vout)
);
CREATE INDEX IF NOT EXISTS utxo_account_i ON utxo (account_address);

CREATE TABLE IF NOT EXISTS chainstate (
	best_hash TEXT NOT NULL,
	best_height INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS address_block (
	address TEXT NOT NULL,
	height INTEGER NOT NULL,
	PRIMARY KEY (address, height)
);
`

/****************** SQLiteStore implements giga.Store ********************/
var _ giga.Store = SQLiteStore{}

type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore returns a giga.PaymentsStore implementor that uses sqlite
func NewSQLiteStore(fileName string) (SQLiteStore, error) {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		return SQLiteStore{}, dbErr(err, "opening database")
	}
	// WAL mode provides more concurrency
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return SQLiteStore{}, dbErr(err, "creating database schema")
	}
	// init tables / indexes
	_, err = db.Exec(SETUP_SQL)
	if err != nil {
		return SQLiteStore{}, dbErr(err, "creating database schema")
	}

	return SQLiteStore{db}, nil
}

// Defer this until shutdown
func (s SQLiteStore) Close() {
	s.db.Close()
}

func (s SQLiteStore) Begin(timeout time.Duration) (giga.StoreTransaction, error) {
	if timeout < MINIMUM_TIMEOUT {
		timeout = MINIMUM_TIMEOUT
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		defer cancel()
		return &SQLiteStoreTransaction{}, err
	}
	return NewStoreTransaction(tx, ctx, cancel), nil
}

func (s SQLiteStore) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), MINIMUM_TIMEOUT)
	defer cancel()
	row := s.db.QueryRowContext(ctx, "SELECT invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE invoice_address = ?", addr)
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

func (s SQLiteStore) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), MINIMUM_TIMEOUT)
	defer cancel()
	// MUST order by key_index to support the cursor API:
	// we need a way to resume the query next time from whatever next_cursor we return,
	// and the aggregate result SHOULD be stable even as the DB is modified.
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := s.db.QueryContext(ctx, "SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ? AND key_index >= ? ORDER BY key_index LIMIT ?", account, cursor, limit)
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

func (s SQLiteStore) GetAccount(foreignID string) (giga.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), MINIMUM_TIMEOUT)
	defer cancel()
	row := s.db.QueryRowContext(ctx, "SELECT foreign_id,address,privkey,next_int_key,next_ext_key,max_pool_int,max_pool_ext FROM account WHERE foreign_id = ?", foreignID)
	var acc giga.Account
	err := row.Scan(
		&acc.ForeignID, &acc.Address, &acc.Privkey,
		&acc.NextInternalKey, &acc.NextExternalKey,
		&acc.MaxPoolInternal, &acc.MaxPoolExternal,
		&acc.PayoutAddress, &acc.PayoutThreshold, &acc.PayoutFrequency) // common (see updateAccount)
	if err == sql.ErrNoRows {
		// MUST detect this error to fulfil the API contract.
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %s", foreignID)
	}
	if err != nil {
		return giga.Account{}, dbErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (s SQLiteStore) GetChainState() (giga.ChainState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), MINIMUM_TIMEOUT)
	defer cancel()
	row := s.db.QueryRowContext(ctx, "SELECT best_hash, best_height FROM chainstate")
	var state giga.ChainState
	err := row.Scan(&state.BestBlockHash, &state.BestBlockHeight)
	if err == sql.ErrNoRows {
		// MUST detect this error to fulfil the API contract.
		return giga.ChainState{}, giga.NewErr(giga.NotFound, "chainstate not found")
	}
	if err != nil {
		return giga.ChainState{}, dbErr(err, "GetChainState: row.Scan")
	}
	return state, nil
}

func (s SQLiteStore) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), MINIMUM_TIMEOUT)
	defer cancel()
	rows_found := 0
	rows, err := s.db.QueryContext(ctx, "SELECT txn_id, vout, value, script_type, script_address FROM utxo WHERE account_address = ? AND status = 'c'", account)
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

/****** SQLiteStoreTransaction implements giga.StoreTransaction ******/
var _ giga.StoreTransaction = &SQLiteStoreTransaction{}

type SQLiteStoreTransaction struct {
	tx       *sql.Tx
	ctx      context.Context
	cancel   context.CancelFunc
	finality bool
}

func NewStoreTransaction(txn *sql.Tx, ctx context.Context, cancel context.CancelFunc) *SQLiteStoreTransaction {
	return &SQLiteStoreTransaction{txn, ctx, cancel, false}
}

func (t *SQLiteStoreTransaction) Commit() error {
	defer t.cancel() // cancel timeout (must be after Commit)
	err := t.tx.Commit()
	if err != nil {
		return err
	}
	t.finality = true
	return nil
}

func (t SQLiteStoreTransaction) Rollback() error {
	if !t.finality {
		defer t.cancel() // cancel timeout (must be after Rollback)
		err := t.tx.Rollback()
		return err
	}
	return nil
}

// Store an invoice
func (t SQLiteStoreTransaction) StoreInvoice(inv giga.Invoice) error {
	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		return dbErr(err, "createInvoice: json.Marshal items")
	}

	stmt, err := t.tx.PrepareContext(t.ctx, "insert into invoice(invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations) values(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return dbErr(err, "createInvoice: tx.Prepare insert")
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(t.ctx, inv.ID, inv.Account, inv.TXID, inv.Vendor, string(items_b), inv.KeyIndex, inv.BlockID, inv.Confirmations)
	if err != nil {
		return dbErr(err, "createInvoice: stmt.Exec insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	row := t.tx.QueryRowContext(t.ctx, "SELECT invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE invoice_address = ?", addr)
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

func (t SQLiteStoreTransaction) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// MUST order by key_index (or sqlite OID) to support the cursor API:
	// we need a way to resume the query next time from whatever next_cursor we return,
	// and the aggregate result SHOULD be stable even as the DB is modified.
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := t.tx.QueryContext(t.ctx, "SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ? AND key_index >= ? ORDER BY key_index LIMIT ?", account, cursor, limit)
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

func (t SQLiteStoreTransaction) CreateAccount(acc giga.Account) error {
	stmt, err := t.tx.PrepareContext(t.ctx, "insert into account(foreign_id,address,privkey,next_int_key,next_ext_key,max_pool_int,max_pool_ext,payout_address,payout_threshold,payout_frequency) values(?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return dbErr(err, "createAccount: preparing insert")
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(t.ctx,
		acc.ForeignID, acc.Address, acc.Privkey, // only in createAccount.
		acc.NextInternalKey, acc.NextExternalKey, // common (see updateAccount) ...
		acc.MaxPoolInternal, acc.MaxPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency)
	if err != nil {
		return dbErr(err, "createAccount: executing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) UpdateAccount(acc giga.Account) error {
	stmt, err := t.tx.PrepareContext(t.ctx, "update account set next_int_key=MAX(next_int_key,?), max_pool_int=MAX(max_pool_int,?), max_pool_ext=MAX(max_pool_ext,?), next_ext_key=MAX(next_ext_key,?), payout_address=?, payout_threshold=?, payout_frequency=? where foreign_id=?")
	if err != nil {
		return dbErr(err, "updateAccount: preparing update")
	}
	defer stmt.Close()
	res, err := stmt.ExecContext(t.ctx,
		acc.NextInternalKey, acc.NextExternalKey, // common (see createAccount) ...
		acc.MaxPoolInternal, acc.MaxPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency,
		acc.ForeignID) // the Key (not updated)
	if err != nil {
		return dbErr(err, "updateAccount: executing update")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return dbErr(err, "updateAccount: res.RowsAffected")
	}
	if num_rows < 1 {
		// MUST detect this error to fulfil the API contract.
		return giga.NewErr(giga.NotFound, "account not found: %s", acc.ForeignID)
	}
	return nil
}

func (t SQLiteStoreTransaction) StoreAddresses(accountID giga.Address, addresses []giga.Address, firstAddress uint32, isInternal bool) error {
	// Associate a list of addresses with an accountID in the account_address table.
	stmt, err := t.tx.PrepareContext(t.ctx, "INSERT INTO account_address (address,key_index,is_internal,account_address) VALUES (?,?,?)")
	if err != nil {
		return dbErr(err, "StoreAddresses: preparing insert")
	}
	defer stmt.Close()
	firstKey := firstAddress
	for n, addr := range addresses {
		_, err = stmt.ExecContext(t.ctx, addr, firstKey+uint32(n), isInternal, accountID)
		if err != nil {
			return dbErr(err, "StoreAddresses: executing insert")
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) GetAccount(foreignID string) (giga.Account, error) {
	row := t.tx.QueryRowContext(t.ctx, "SELECT foreign_id, address, privkey, next_int_key, next_ext_key FROM account WHERE foreign_id = ?", foreignID)
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

func (t SQLiteStoreTransaction) FindAccountForAddress(address giga.Address) (giga.Address, uint32, bool, error) {
	row := t.tx.QueryRowContext(t.ctx, "SELECT account_address,key_index,is_internal FROM account_address WHERE address = ?", address)
	var accountID giga.Address
	var keyIndex uint32
	var isInternal bool
	err := row.Scan(&accountID, &keyIndex, &isInternal)
	if err == sql.ErrNoRows {
		return "", 0, false, giga.NewErr(giga.NotFound, "no matching account for address: %s", address)
	}
	if err != nil {
		return "", 0, false, dbErr(err, "FindAccountForAddress: error scanning row")
	}
	return accountID, keyIndex, isInternal, nil
}

func (t SQLiteStoreTransaction) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	rows_found := 0
	rows, err := t.tx.QueryContext(t.ctx, "SELECT txn_id, vout, value, script_type, script_address FROM utxo WHERE account_address = ? AND status = 'c'", account)
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

func (t SQLiteStoreTransaction) MarkInvoiceAsPaid(address giga.Address) error {
	stmt, err := t.tx.PrepareContext(t.ctx, "update invoices set confirmations = ? where invoice_address = ?")
	if err != nil {
		return dbErr(err, "markInvoiceAsPaid: preparing update")
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(t.ctx, 1, address)
	if err != nil {
		return dbErr(err, "markInvoiceAsPaid: executing update")
	}
	return nil
}

func (t SQLiteStoreTransaction) UpdateChainState(state giga.ChainState) error {
	stmt, err := t.tx.PrepareContext(t.ctx, "UPDATE chainstate SET best_hash = ?, best_height = ?")
	if err != nil {
		return dbErr(err, "UpdateChainState: preparing update")
	}
	defer stmt.Close()
	res, err := stmt.ExecContext(t.ctx, state.BestBlockHash, state.BestBlockHeight)
	if err != nil {
		return dbErr(err, "UpdateChainState: executing update")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return dbErr(err, "UpdateChainState: res.RowsAffected")
	}
	if num_rows < 1 {
		// this is the first call to UpdateChainState: insert the row.
		stmt, err := t.tx.PrepareContext(t.ctx, "INSERT INTO chainstate (best_hash, best_height) VALUES (?, ?)")
		if err != nil {
			return dbErr(err, "UpdateChainState: preparing insert")
		}
		defer stmt.Close()
		_, err = stmt.ExecContext(t.ctx, state.BestBlockHash, state.BestBlockHeight)
		if err != nil {
			return dbErr(err, "UpdateChainState: executing insert")
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) CreateUTXO(txID string, vOut int64, value giga.CoinAmount, scriptType string, pkhAddress giga.Address, accountID giga.Address, keyIndex uint32, isInternal bool, blockHeight int64) error {
	stmt, err := t.tx.PrepareContext(t.ctx, "INSERT INTO utxo (txn_id, vout, account_address, value, script_type, script_address, key_index, is_internal, adding_height)")
	if err != nil {
		return dbErr(err, "CreateUTXO: preparing insert")
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(t.ctx, txID, vOut, accountID, value, scriptType, pkhAddress, keyIndex, isInternal, blockHeight)
	if err != nil {
		return dbErr(err, "CreateUTXO: executing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) MarkUTXOSpent(txID string, vOut int64, blockHeight int64) error {
	stmt, err := t.tx.PrepareContext(t.ctx, "UPDATE utxo SET spending_height = ? WHERE txn_id = ? AND vout = ?")
	if err != nil {
		return dbErr(err, "MarkUTXOSpent: preparing update")
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(t.ctx, blockHeight, txID, vOut)
	if err != nil {
		return dbErr(err, "MarkUTXOSpent: executing update")
	}
	return nil
}

func (t SQLiteStoreTransaction) RevertUTXOsAboveHeight(maxValidHeight int64) error {
	// The presence of a height in adding_height, available_height, spending_height or spent_height
	// indicates that the UTXO is in the process of being added, or has been added (confirmed); is
	// reserved for spending, or has been spent (confirmed)
	stmt1, err := t.tx.PrepareContext(t.ctx, "UPDATE utxo SET adding_height = NULL, available_height = NULL, spending_height = NULL, spent_height = NULL, dirty = true WHERE adding_height > ?")
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: preparing update 1")
	}
	defer stmt1.Close()
	stmt2, err := t.tx.PrepareContext(t.ctx, "UPDATE utxo SET available_height = NULL, spending_height = NULL, spent_height = NULL, dirty = true WHERE available_height > ?")
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: preparing update 2")
	}
	defer stmt2.Close()
	stmt3, err := t.tx.PrepareContext(t.ctx, "UPDATE utxo SET spending_height = NULL, spent_height = NULL, dirty = true WHERE spending_height > ?")
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: preparing update 3")
	}
	defer stmt3.Close()
	stmt4, err := t.tx.PrepareContext(t.ctx, "UPDATE utxo SET spent_height = NULL, dirty = true WHERE spent_height > ?")
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: preparing update 4")
	}
	defer stmt4.Close()
	_, err = stmt1.ExecContext(t.ctx, maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 1")
	}
	_, err = stmt2.ExecContext(t.ctx, maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 2")
	}
	_, err = stmt3.ExecContext(t.ctx, maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 3")
	}
	_, err = stmt4.ExecContext(t.ctx, maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 4")
	}
	return nil
}

func (t SQLiteStoreTransaction) RevertTxnsAboveHeight(maxValidHeight int64) error {
	// The presence of a height in on_chain_height or verified_height indicates
	// that the invoice is in a block (on-chain) or has been verified (N blocks later)
	stmt1, err := t.tx.PrepareContext(t.ctx, "UPDATE invoice SET on_chain_height = NULL, verified_height = NULL, dirty = true WHERE on_chain_height > ?")
	if err != nil {
		return dbErr(err, "RevertTxnsAboveHeight: preparing update 1")
	}
	defer stmt1.Close()
	stmt2, err := t.tx.PrepareContext(t.ctx, "UPDATE invoice SET verified_height = NULL, dirty = true WHERE verified_height > ?")
	if err != nil {
		return dbErr(err, "RevertTxnsAboveHeight: preparing update 2")
	}
	defer stmt2.Close()
	_, err = stmt1.ExecContext(t.ctx, maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertTxnsAboveHeight: executing update 1")
	}
	_, err = stmt2.ExecContext(t.ctx, maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertTxnsAboveHeight: executing update 2")
	}
	return nil
}

func dbErr(err error, where string) error {
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
