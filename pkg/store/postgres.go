package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/shopspring/decimal"

	"github.com/lib/pq"
)

const SET_UP_POSTGRES string = `
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

/****************** PostgresStore implements giga.Store ********************/
var _ giga.Store = PostgresStore{}

type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore returns a giga.PaymentsStore implementor that uses sqlite
func NewPostgresStore(fileName string) (PostgresStore, error) {
	// fileName: "postgres://user:password@localhost/db_name"
	db, err := sql.Open("postgres", fileName)
	if err != nil {
		return PostgresStore{}, pqErr(err, "opening database")
	}
	// init tables / indexes
	_, err = db.Exec(SET_UP_POSTGRES)
	if err != nil {
		db.Close()
		return PostgresStore{}, pqErr(err, "creating database schema")
	}
	return PostgresStore{db}, nil
}

// Defer this until shutdown
func (s PostgresStore) Close() {
	s.db.Close()
}

func (s PostgresStore) Begin() (giga.StoreTransaction, error) {
	// Use 'Serializable' isolation so we don't need to reason about anomalies.
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return &PostgresStoreTransaction{}, err
	}
	return &PostgresStoreTransaction{tx, false}, nil
}

func (s PostgresStore) GetInvoice(addr giga.Address) (giga.Invoice, error) {
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
		return giga.Invoice{}, pqErr(err, "GetInvoice: row.Scan")
	}
	var items []giga.Item
	err = json.Unmarshal([]byte(items_json), &items)
	if err != nil {
		return giga.Invoice{}, pqErr(err, "GetInvoice: json.Unmarshal")
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

func (s PostgresStore) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// MUST order by key_index to support the cursor API:
	// we need a way to resume the query next time from whatever next_cursor we return,
	// and the aggregate result SHOULD be stable even as the DB is modified.
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := s.db.Query("SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ? AND key_index >= ? ORDER BY key_index LIMIT ?", account, cursor, limit)
	if err != nil {
		return nil, 0, pqErr(err, "ListInvoices: querying invoices")
	}
	defer rows.Close()
	for rows.Next() {
		inv := giga.Invoice{Account: account}
		var items_json string
		err := rows.Scan(&inv.ID, &inv.TXID, &inv.Vendor, &items_json, &inv.KeyIndex, &inv.BlockID, &inv.Confirmations)
		if err != nil {
			return nil, 0, pqErr(err, "ListInvoices: scanning invoice row")
		}
		err = json.Unmarshal([]byte(items_json), &inv.Items)
		if err != nil {
			return nil, 0, pqErr(err, "ListInvoices: unmarshalling json")
		}
		items = append(items, inv)
		after_this := int(inv.KeyIndex) + 1 // XXX assumes non-hardened HD Key! (from uint32)
		if after_this > next_cursor {
			next_cursor = after_this // NB. starting cursor for next call
		}
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, 0, pqErr(err, "ListInvoices: querying invoices")
	}
	if rows_found < limit {
		// in this backend, we know there are no more rows to follow.
		next_cursor = 0 // meaning "end of query results"
	}
	return
}

func (s PostgresStore) GetAccount(foreignID string) (giga.Account, error) {
	row := s.db.QueryRow("SELECT foreign_id,address,privkey,next_int_key,next_ext_key,max_pool_int,max_pool_ext FROM account WHERE foreign_id = ?", foreignID)
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
		return giga.Account{}, pqErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (s PostgresStore) GetChainState() (giga.ChainState, error) {
	row := s.db.QueryRow("SELECT best_hash, best_height FROM chainstate")
	var state giga.ChainState
	err := row.Scan(&state.BestBlockHash, &state.BestBlockHeight)
	if err == sql.ErrNoRows {
		// MUST detect this error to fulfil the API contract.
		return giga.ChainState{}, giga.NewErr(giga.NotFound, "chainstate not found")
	}
	if err != nil {
		return giga.ChainState{}, pqErr(err, "GetChainState: row.Scan")
	}
	return state, nil
}

func (s PostgresStore) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	rows_found := 0
	rows, err := s.db.Query("SELECT txn_id, vout, value, script_type, script_address FROM utxo WHERE account_address = ? AND status = 'c'", account)
	if err != nil {
		return nil, pqErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	defer rows.Close()
	for rows.Next() {
		utxo := giga.UTXO{Account: account, Status: "c"}
		var value string
		err := rows.Scan(&utxo.TxnID, &utxo.VOut, &value, &utxo.ScriptType, &utxo.ScriptAddress)
		if err != nil {
			return nil, pqErr(err, "GetAllUnreservedUTXOs: scanning UTXO row")
		}
		utxo.Value, err = decimal.NewFromString(value)
		if err != nil {
			return nil, pqErr(err, fmt.Sprintf("GetAllUnreservedUTXOs: invalid decimal value in UTXO database: %v", value))
		}
		result = append(result, utxo)
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, pqErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	return
}

/****** PostgresStoreTransaction implements giga.StoreTransaction ******/
var _ giga.StoreTransaction = &PostgresStoreTransaction{}

type PostgresStoreTransaction struct {
	tx       *sql.Tx
	finality bool
}

func (t *PostgresStoreTransaction) Commit() error {
	err := t.tx.Commit()
	if err != nil {
		return err
	}
	t.finality = true
	return nil
}

func (t PostgresStoreTransaction) Rollback() error {
	if !t.finality {
		return t.tx.Rollback()
	}
	return nil
}

// Store an invoice
func (t PostgresStoreTransaction) StoreInvoice(inv giga.Invoice) error {
	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		return pqErr(err, "createInvoice: json.Marshal items")
	}

	stmt, err := t.tx.Prepare("insert into invoice(invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations) values(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return pqErr(err, "createInvoice: tx.Prepare insert")
	}
	defer stmt.Close()

	_, err = stmt.Exec(inv.ID, inv.Account, inv.TXID, inv.Vendor, string(items_b), inv.KeyIndex, inv.BlockID, inv.Confirmations)
	if err != nil {
		return pqErr(err, "createInvoice: stmt.Exec insert")
	}
	return nil
}

func (t PostgresStoreTransaction) GetInvoice(addr giga.Address) (giga.Invoice, error) {
	row := t.tx.QueryRow("SELECT invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE invoice_address = ?", addr)
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
		return giga.Invoice{}, pqErr(err, "GetInvoice: row.Scan")
	}
	var items []giga.Item
	err = json.Unmarshal([]byte(items_json), &items)
	if err != nil {
		return giga.Invoice{}, pqErr(err, "GetInvoice: json.Unmarshal")
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

func (t PostgresStoreTransaction) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// MUST order by key_index (or sqlite OID) to support the cursor API:
	// we need a way to resume the query next time from whatever next_cursor we return,
	// and the aggregate result SHOULD be stable even as the DB is modified.
	// note: we CAN return less than 'limit' items on each call, and there can be gaps (e.g. filtering)
	rows_found := 0
	rows, err := t.tx.Query("SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ? AND key_index >= ? ORDER BY key_index LIMIT ?", account, cursor, limit)
	if err != nil {
		return nil, 0, pqErr(err, "ListInvoices: querying invoices")
	}
	defer rows.Close()
	for rows.Next() {
		inv := giga.Invoice{Account: account}
		var items_json string
		err := rows.Scan(&inv.ID, &inv.TXID, &inv.Vendor, &items_json, &inv.KeyIndex, &inv.BlockID, &inv.Confirmations)
		if err != nil {
			return nil, 0, pqErr(err, "ListInvoices: scanning invoice row")
		}
		err = json.Unmarshal([]byte(items_json), &inv.Items)
		if err != nil {
			return nil, 0, pqErr(err, "ListInvoices: unmarshalling json")
		}
		items = append(items, inv)
		after_this := int(inv.KeyIndex) + 1 // XXX assumes non-hardened HD Key! (from uint32)
		if after_this > next_cursor {
			next_cursor = after_this // NB. starting cursor for next call
		}
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, 0, pqErr(err, "ListInvoices: querying invoices")
	}
	if rows_found < limit {
		// in this backend, we know there are no more rows to follow.
		next_cursor = 0 // meaning "end of query results"
	}
	return
}

func (t PostgresStoreTransaction) CreateAccount(acc giga.Account) error {
	stmt, err := t.tx.Prepare("insert into account(foreign_id,address,privkey,next_int_key,next_ext_key,max_pool_int,max_pool_ext,payout_address,payout_threshold,payout_frequency) values(?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return pqErr(err, "createAccount: preparing insert")
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		acc.ForeignID, acc.Address, acc.Privkey, // only in createAccount.
		acc.NextInternalKey, acc.NextExternalKey, // common (see updateAccount) ...
		acc.MaxPoolInternal, acc.MaxPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency)
	if err != nil {
		return pqErr(err, "createAccount: executing insert")
	}
	return nil
}

func (t PostgresStoreTransaction) UpdateAccount(acc giga.Account) error {
	stmt, err := t.tx.Prepare("update account set next_int_key=MAX(next_int_key,?), max_pool_int=MAX(max_pool_int,?), max_pool_ext=MAX(max_pool_ext,?), next_ext_key=MAX(next_ext_key,?), payout_address=?, payout_threshold=?, payout_frequency=? where foreign_id=?")
	if err != nil {
		return pqErr(err, "updateAccount: preparing update")
	}
	defer stmt.Close()
	res, err := stmt.Exec(
		acc.NextInternalKey, acc.NextExternalKey, // common (see createAccount) ...
		acc.MaxPoolInternal, acc.MaxPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency,
		acc.ForeignID) // the Key (not updated)
	if err != nil {
		return pqErr(err, "updateAccount: executing update")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return pqErr(err, "updateAccount: res.RowsAffected")
	}
	if num_rows < 1 {
		// MUST detect this error to fulfil the API contract.
		return giga.NewErr(giga.NotFound, "account not found: %s", acc.ForeignID)
	}
	return nil
}

func (t PostgresStoreTransaction) StoreAddresses(accountID giga.Address, addresses []giga.Address, firstAddress uint32, isInternal bool) error {
	// Associate a list of addresses with an accountID in the account_address table.
	stmt, err := t.tx.Prepare("INSERT INTO account_address (address,key_index,is_internal,account_address) VALUES (?,?,?)")
	if err != nil {
		return pqErr(err, "StoreAddresses: preparing insert")
	}
	defer stmt.Close()
	firstKey := firstAddress
	for n, addr := range addresses {
		_, err = stmt.Exec(addr, firstKey+uint32(n), isInternal, accountID)
		if err != nil {
			return pqErr(err, "StoreAddresses: executing insert")
		}
	}
	return nil
}

func (t PostgresStoreTransaction) GetAccount(foreignID string) (giga.Account, error) {
	row := t.tx.QueryRow("SELECT foreign_id, address, privkey, next_int_key, next_ext_key FROM account WHERE foreign_id = ?", foreignID)
	var acc giga.Account
	err := row.Scan(&acc.ForeignID, &acc.Address, &acc.Privkey, &acc.NextInternalKey, &acc.NextExternalKey)
	if err == sql.ErrNoRows {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %s", foreignID)
	}
	if err != nil {
		return giga.Account{}, pqErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (t PostgresStoreTransaction) FindAccountForAddress(address giga.Address) (giga.Address, uint32, bool, error) {
	row := t.tx.QueryRow("SELECT account_address,key_index,is_internal FROM account_address WHERE address = ?", address)
	var accountID giga.Address
	var keyIndex uint32
	var isInternal bool
	err := row.Scan(&accountID, &keyIndex, &isInternal)
	if err == sql.ErrNoRows {
		return "", 0, false, giga.NewErr(giga.NotFound, "no matching account for address: %s", address)
	}
	if err != nil {
		return "", 0, false, pqErr(err, "FindAccountForAddress: error scanning row")
	}
	return accountID, keyIndex, isInternal, nil
}

func (t PostgresStoreTransaction) GetAllUnreservedUTXOs(account giga.Address) (result []giga.UTXO, err error) {
	rows_found := 0
	rows, err := t.tx.Query("SELECT txn_id, vout, value, script_type, script_address FROM utxo WHERE account_address = ? AND status = 'c'", account)
	if err != nil {
		return nil, pqErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	defer rows.Close()
	for rows.Next() {
		utxo := giga.UTXO{Account: account, Status: "c"}
		var value string
		err := rows.Scan(&utxo.TxnID, &utxo.VOut, &value, &utxo.ScriptType, &utxo.ScriptAddress)
		if err != nil {
			return nil, pqErr(err, "GetAllUnreservedUTXOs: scanning UTXO row")
		}
		utxo.Value, err = decimal.NewFromString(value)
		if err != nil {
			return nil, pqErr(err, fmt.Sprintf("GetAllUnreservedUTXOs: invalid decimal value in UTXO database: %v", value))
		}
		result = append(result, utxo)
		rows_found++
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return nil, pqErr(err, "GetAllUnreservedUTXOs: querying UTXOs")
	}
	return
}

func (t PostgresStoreTransaction) MarkInvoiceAsPaid(address giga.Address) error {
	stmt, err := t.tx.Prepare("update invoices set confirmations = ? where invoice_address = ?")
	if err != nil {
		return pqErr(err, "markInvoiceAsPaid: preparing update")
	}
	defer stmt.Close()
	_, err = stmt.Exec(1, address)
	if err != nil {
		return pqErr(err, "markInvoiceAsPaid: executing update")
	}
	return nil
}

func (t PostgresStoreTransaction) UpdateChainState(state giga.ChainState) error {
	stmt, err := t.tx.Prepare("UPDATE chainstate SET best_hash = ?, best_height = ?")
	if err != nil {
		return pqErr(err, "UpdateChainState: preparing update")
	}
	defer stmt.Close()
	res, err := stmt.Exec(state.BestBlockHash, state.BestBlockHeight)
	if err != nil {
		return pqErr(err, "UpdateChainState: executing update")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return pqErr(err, "UpdateChainState: res.RowsAffected")
	}
	if num_rows < 1 {
		// this is the first call to UpdateChainState: insert the row.
		stmt, err := t.tx.Prepare("INSERT INTO chainstate (best_hash, best_height) VALUES (?, ?)")
		if err != nil {
			return pqErr(err, "UpdateChainState: preparing insert")
		}
		defer stmt.Close()
		_, err = stmt.Exec(state.BestBlockHash, state.BestBlockHeight)
		if err != nil {
			return pqErr(err, "UpdateChainState: executing insert")
		}
	}
	return nil
}

func (t PostgresStoreTransaction) CreateUTXO(txID string, vOut int64, value giga.CoinAmount, scriptType string, pkhAddress giga.Address, accountID giga.Address, keyIndex uint32, isInternal bool, blockHeight int64) error {
	stmt, err := t.tx.Prepare("INSERT INTO utxo (txn_id, vout, account_address, value, script_type, script_address, key_index, is_internal, adding_height)")
	if err != nil {
		return pqErr(err, "CreateUTXO: preparing insert")
	}
	defer stmt.Close()
	_, err = stmt.Exec(txID, vOut, accountID, value, scriptType, pkhAddress, keyIndex, isInternal, blockHeight)
	if err != nil {
		return pqErr(err, "CreateUTXO: executing insert")
	}
	return nil
}

func (t PostgresStoreTransaction) MarkUTXOSpent(txID string, vOut int64, blockHeight int64) error {
	stmt, err := t.tx.Prepare("UPDATE utxo SET spending_height = ? WHERE txn_id = ? AND vout = ?")
	if err != nil {
		return pqErr(err, "MarkUTXOSpent: preparing update")
	}
	defer stmt.Close()
	_, err = stmt.Exec(blockHeight, txID, vOut)
	if err != nil {
		return pqErr(err, "MarkUTXOSpent: executing update")
	}
	return nil
}

func (t PostgresStoreTransaction) RevertUTXOsAboveHeight(maxValidHeight int64) error {
	// The presence of a height in adding_height, available_height, spending_height or spent_height
	// indicates that the UTXO is in the process of being added, or has been added (confirmed); is
	// reserved for spending, or has been spent (confirmed)
	stmt1, err := t.tx.Prepare("UPDATE utxo SET adding_height = NULL, available_height = NULL, spending_height = NULL, spent_height = NULL, dirty = true WHERE adding_height > ?")
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: preparing update 1")
	}
	defer stmt1.Close()
	stmt2, err := t.tx.Prepare("UPDATE utxo SET available_height = NULL, spending_height = NULL, spent_height = NULL, dirty = true WHERE available_height > ?")
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: preparing update 2")
	}
	defer stmt2.Close()
	stmt3, err := t.tx.Prepare("UPDATE utxo SET spending_height = NULL, spent_height = NULL, dirty = true WHERE spending_height > ?")
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: preparing update 3")
	}
	defer stmt3.Close()
	stmt4, err := t.tx.Prepare("UPDATE utxo SET spent_height = NULL, dirty = true WHERE spent_height > ?")
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: preparing update 4")
	}
	defer stmt4.Close()
	_, err = stmt1.Exec(maxValidHeight)
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: executing update 1")
	}
	_, err = stmt2.Exec(maxValidHeight)
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: executing update 2")
	}
	_, err = stmt3.Exec(maxValidHeight)
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: executing update 3")
	}
	_, err = stmt4.Exec(maxValidHeight)
	if err != nil {
		return pqErr(err, "RevertUTXOsAboveHeight: executing update 4")
	}
	return nil
}

func (t PostgresStoreTransaction) RevertTxnsAboveHeight(maxValidHeight int64) error {
	// The presence of a height in on_chain_height or verified_height indicates
	// that the invoice is in a block (on-chain) or has been verified (N blocks later)
	stmt1, err := t.tx.Prepare("UPDATE invoice SET on_chain_height = NULL, verified_height = NULL, dirty = true WHERE on_chain_height > ?")
	if err != nil {
		return pqErr(err, "RevertTxnsAboveHeight: preparing update 1")
	}
	defer stmt1.Close()
	stmt2, err := t.tx.Prepare("UPDATE invoice SET verified_height = NULL, dirty = true WHERE verified_height > ?")
	if err != nil {
		return pqErr(err, "RevertTxnsAboveHeight: preparing update 2")
	}
	defer stmt2.Close()
	_, err = stmt1.Exec(maxValidHeight)
	if err != nil {
		return pqErr(err, "RevertTxnsAboveHeight: executing update 1")
	}
	_, err = stmt2.Exec(maxValidHeight)
	if err != nil {
		return pqErr(err, "RevertTxnsAboveHeight: executing update 2")
	}
	return nil
}

func pqErr(err error, where string) error {
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
}
