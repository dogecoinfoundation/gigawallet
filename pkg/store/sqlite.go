package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
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
	payout_threshold TEXT NOT NULL,
	payout_frequency TEXT NOT NULL,
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
	key_index INTEGER NOT NULL,
	block_id TEXT NOT NULL,
	confirmations INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS invoice_account_i ON invoice (account_address);

CREATE TABLE IF NOT EXISTS txn (
	txn_id TEXT NOT NULL PRIMARY KEY,
	account_address TEXT NOT NULL,
	invoice_address TEXT,
	on_chain_height INTEGER,
	verified_height INTEGER,
	send_verified BOOLEAN NOT NULL DEFAULT false,
	send_rollback BOOLEAN NOT NULL DEFAULT false
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
CREATE INDEX IF NOT EXISTS utxo_added_i ON utxo (adding_height, available_height);
CREATE INDEX IF NOT EXISTS utxo_spent_i ON utxo (spending_height, spent_height);

CREATE TABLE IF NOT EXISTS chainstate (
	root_hash TEXT NOT NULL,
	first_height INTEGER NOT NULL,
	best_hash TEXT NOT NULL,
	best_height INTEGER NOT NULL
);
`

/****************** SQLiteStore implements giga.Store ********************/
var _ giga.Store = SQLiteStore{}

type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore returns a giga.PaymentsStore implementor that uses sqlite
func NewSQLiteStore(fileName string) (giga.Store, error) {
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

func (s SQLiteStore) Begin() (giga.StoreTransaction, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return &SQLiteStoreTransaction{}, err
	}
	return NewStoreTransaction(tx), nil
}

func (s SQLiteStore) GetInvoice(addr giga.Address) (giga.Invoice, error) {
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

func (s SQLiteStore) ListInvoices(account giga.Address, cursor int, limit int) (items []giga.Invoice, next_cursor int, err error) {
	// MUST order by key_index to support the cursor API:
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

func (s SQLiteStore) GetAccount(foreignID string) (giga.Account, error) {
	row := s.db.QueryRow("SELECT foreign_id,address,privkey,next_int_key,next_ext_key,next_pool_int,next_pool_ext,payout_address,payout_threshold,payout_frequency FROM account WHERE foreign_id = ?", foreignID)
	var acc giga.Account
	err := row.Scan(
		&acc.ForeignID, &acc.Address, &acc.Privkey,
		&acc.NextInternalKey, &acc.NextExternalKey,
		&acc.NextPoolInternal, &acc.NextPoolExternal,
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
	row := s.db.QueryRow("SELECT best_hash, best_height, root_hash, first_height FROM chainstate")
	var state giga.ChainState
	err := row.Scan(&state.BestBlockHash, &state.BestBlockHeight, &state.RootHash, &state.FirstHeight)
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

/****** SQLiteStoreTransaction implements giga.StoreTransaction ******/
var _ giga.StoreTransaction = &SQLiteStoreTransaction{}

type SQLiteStoreTransaction struct {
	tx       *sql.Tx
	finality bool
}

func NewStoreTransaction(txn *sql.Tx) *SQLiteStoreTransaction {
	return &SQLiteStoreTransaction{txn, false}
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

// Store an invoice
func (t SQLiteStoreTransaction) StoreInvoice(inv giga.Invoice) error {
	items_b, err := json.Marshal(inv.Items)
	if err != nil {
		return dbErr(err, "createInvoice: json.Marshal items")
	}

	stmt, err := t.tx.Prepare("insert into invoice(invoice_address, account_address, txn_id, vendor, items, key_index, block_id, confirmations) values(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return dbErr(err, "createInvoice: tx.Prepare insert")
	}
	defer stmt.Close()

	_, err = stmt.Exec(inv.ID, inv.Account, inv.TXID, inv.Vendor, string(items_b), inv.KeyIndex, inv.BlockID, inv.Confirmations)
	if err != nil {
		return dbErr(err, "createInvoice: stmt.Exec insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) GetInvoice(addr giga.Address) (giga.Invoice, error) {
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
	rows, err := t.tx.Query("SELECT invoice_address, txn_id, vendor, items, key_index, block_id, confirmations FROM invoice WHERE account_address = ? AND key_index >= ? ORDER BY key_index LIMIT ?", account, cursor, limit)
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
	stmt, err := t.tx.Prepare("insert into account(foreign_id,address,privkey,next_int_key,next_ext_key,next_pool_int,next_pool_ext,payout_address,payout_threshold,payout_frequency) values(?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return dbErr(err, "createAccount: preparing insert")
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		acc.ForeignID, acc.Address, acc.Privkey, // only in createAccount.
		acc.NextInternalKey, acc.NextExternalKey, // common (see updateAccount) ...
		acc.NextPoolInternal, acc.NextPoolExternal,
		acc.PayoutAddress, acc.PayoutThreshold, acc.PayoutFrequency)
	if err != nil {
		return dbErr(err, "createAccount: executing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) UpdateAccount(acc giga.Account) error {
	stmt, err := t.tx.Prepare("update account set next_int_key=MAX(next_int_key,?), next_ext_key=MAX(next_ext_key,?), next_pool_int=MAX(next_pool_int,?), next_pool_ext=MAX(next_pool_ext,?), payout_address=?, payout_threshold=?, payout_frequency=? where foreign_id=?")
	if err != nil {
		return dbErr(err, "updateAccount: preparing update")
	}
	defer stmt.Close()
	res, err := stmt.Exec(
		acc.NextInternalKey, acc.NextExternalKey, // common (see createAccount) ...
		acc.NextPoolInternal, acc.NextPoolExternal,
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
	stmt, err := t.tx.Prepare("INSERT INTO account_address (address,key_index,is_internal,account_address) VALUES (?,?,?,?)")
	if err != nil {
		return dbErr(err, "StoreAddresses: preparing insert")
	}
	defer stmt.Close()
	firstKey := firstAddress
	for n, addr := range addresses {
		_, err = stmt.Exec(addr, firstKey+uint32(n), isInternal, accountID)
		if err != nil {
			return dbErr(err, "StoreAddresses: executing insert")
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) GetAccount(foreignID string) (giga.Account, error) {
	row := t.tx.QueryRow("SELECT foreign_id,address,privkey,next_int_key,next_ext_key,next_pool_int,next_pool_ext,payout_address,payout_threshold,payout_frequency FROM account WHERE foreign_id = ?", foreignID)
	var acc giga.Account
	err := row.Scan(
		&acc.ForeignID, &acc.Address, &acc.Privkey,
		&acc.NextInternalKey, &acc.NextExternalKey,
		&acc.NextPoolInternal, &acc.NextPoolExternal,
		&acc.PayoutAddress, &acc.PayoutThreshold, &acc.PayoutFrequency) // common (see updateAccount)
	if err == sql.ErrNoRows {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %s", foreignID)
	}
	if err != nil {
		return giga.Account{}, dbErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (t SQLiteStoreTransaction) GetAccountByID(ID string) (giga.Account, error) {
	row := t.tx.QueryRow("SELECT foreign_id,address,privkey,next_int_key,next_ext_key,next_pool_int,next_pool_ext,payout_address,payout_threshold,payout_frequency FROM account WHERE address = ?", ID)
	var acc giga.Account
	err := row.Scan(
		&acc.ForeignID, &acc.Address, &acc.Privkey,
		&acc.NextInternalKey, &acc.NextExternalKey,
		&acc.NextPoolInternal, &acc.NextPoolExternal,
		&acc.PayoutAddress, &acc.PayoutThreshold, &acc.PayoutFrequency) // common (see updateAccount)
	if err == sql.ErrNoRows {
		return giga.Account{}, giga.NewErr(giga.NotFound, "account not found: %s", ID)
	}
	if err != nil {
		return giga.Account{}, dbErr(err, "GetAccount: row.Scan")
	}
	return acc, nil
}

func (t SQLiteStoreTransaction) FindAccountForAddress(address giga.Address) (giga.Address, uint32, bool, error) {
	row := t.tx.QueryRow("SELECT account_address,key_index,is_internal FROM account_address WHERE address = ?", address)
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
	rows, err := t.tx.Query("SELECT txn_id, vout, value, script_type, script_address FROM utxo WHERE account_address = ? AND status = 'c'", account)
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
	stmt, err := t.tx.Prepare("update invoices set confirmations = ? where invoice_address = ?")
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

func (t SQLiteStoreTransaction) UpdateChainState(state giga.ChainState, writeRoot bool) error {
	var res sql.Result
	var err error
	if writeRoot {
		res, err = t.tx.Exec("UPDATE chainstate SET best_hash=$1, best_height=$2, root_hash=$3, first_height=$4", state.BestBlockHash, state.BestBlockHeight, state.RootHash, state.FirstHeight)
	} else {
		res, err = t.tx.Exec("UPDATE chainstate SET best_hash=$1, best_height=$2", state.BestBlockHash, state.BestBlockHeight)
	}
	if err != nil {
		return dbErr(err, "UpdateChainState: executing update")
	}
	num_rows, err := res.RowsAffected()
	if err != nil {
		return dbErr(err, "UpdateChainState: res.RowsAffected")
	}
	if num_rows < 1 {
		// this is the first call to UpdateChainState: insert the row.
		_, err = t.tx.Exec("INSERT INTO chainstate (best_hash,best_height,root_hash,first_height) VALUES ($1,$2,$3,$4)", state.BestBlockHash, state.BestBlockHeight, state.RootHash, state.FirstHeight)
		if err != nil {
			return dbErr(err, "UpdateChainState: executing insert")
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) CreateUTXO(txID string, vOut int64, value giga.CoinAmount, scriptType string, pkhAddress giga.Address, accountID giga.Address, keyIndex uint32, isInternal bool, blockHeight int64) error {
	// psql: "ON CONFLICT ON CONSTRAINT utxo_pkey DO"
	_, err := t.tx.Exec("INSERT INTO utxo (txn_id, vout, account_address, value, script_type, script_address, key_index, is_internal, adding_height) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO UPDATE SET account_address=$3, value=$4, script_type=$5, script_address=$6, key_index=$7, is_internal=$8, adding_height=$9 WHERE txn_id=$1 AND vout=$2",
		txID, vOut, accountID, value, scriptType, pkhAddress, keyIndex, isInternal, blockHeight)
	if err != nil {
		return dbErr(err, "CreateUTXO: executing insert")
	}
	return nil
}

func (t SQLiteStoreTransaction) MarkUTXOSpent(txID string, vOut int64, blockHeight int64) (id string, scriptAddress giga.Address, err error) {
	rows, err := t.tx.Query("UPDATE utxo SET spending_height=$1 WHERE txn_id=$2 AND vout=$3 RETURNING account_address, script_address", blockHeight, txID, vOut)
	if err != nil {
		return "", "", dbErr(err, "MarkUTXOSpent: executing update")
	}
	defer rows.Close()
	if rows.Next() {
		err := rows.Scan(&id, &scriptAddress)
		if err != nil {
			return "", "", dbErr(err, "MarkUTXOSpent: scanning row")
		}
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return "", "", dbErr(err, "MarkUTXOSpent: scanning rows")
	}
	return
}

func (t SQLiteStoreTransaction) IncChainSeqForAccounts(accountIds []string) error {
	if len(accountIds) > 0 {
		// Go's SQL package doesn't implement array/slice arguments,
		// so to do an 'IN' query we need this kind of nonsense.
		binds := strings.Repeat(",?", len(accountIds))[1:]
		args := []any{}
		for _, id := range accountIds {
			args = append(args, id)
		}
		_, err := t.tx.Exec("UPDATE account SET chain_seq=chain_seq+1 WHERE address IN ("+binds+")", args...)
		if err != nil {
			return dbErr(err, "IncAccountChainSeq: executing update")
		}
	}
	return nil
}

func (t SQLiteStoreTransaction) IncAccountsAffectedByRollback(maxValidHeight int64) ([]string, error) {
	rows, err := t.tx.Query(`
		SELECT DISTINCT address FROM account INNER JOIN utxo ON account.address = utxo.account_address WHERE utxo.adding_height > $1 OR utxo.available_height > $1 OR utxo.spending_height > $1 OR utxo.spent_height > $1
		UNION SELECT DISTINCT address FROM account INNER JOIN txn ON account.address = txn.account_address WHERE txn.on_chain_height > $1 OR txn.verified_height > $1`, maxValidHeight)
	if err != nil {
		return []string{}, dbErr(err, "IncAccountsAffectedByRollback: executing query")
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return []string{}, dbErr(err, "IncAccountsAffectedByRollback: scanning row")
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return []string{}, dbErr(err, "IncAccountsAffectedByRollback: scanning rows")
	}
	err = t.IncChainSeqForAccounts(ids)
	return ids, err
}

func (t SQLiteStoreTransaction) ConfirmUTXOs(confirmations int, blockHeight int64) error {
	stmt, err := t.tx.Prepare("UPDATE account SET incoming=incoming-$2, balance=balance+$2 WHERE address=$1")
	if err != nil {
		return dbErr(err, "ConfirmUTXOs: preparing update")
	}
	defer stmt.Close()
	confirmedHeight := blockHeight - int64(confirmations) // from config.
	// note: there is an index on (adding_height, available_height) for this query.
	// note: this uses num-confirmations from the invoice being paid, if there is one.
	// note: this MUST be a LEFT OUTER join (script_address may not match any invoice)
	rows, err := t.tx.Query(`
		UPDATE utxo SET available_height=$1
		WHERE adding_height <= COALESCE((SELECT $1 - confirmations from invoice WHERE invoice_address = utxo.script_address), $2)
		AND available_height = NULL
		RETURNING account_address, value
	`, blockHeight, confirmedHeight)
	if err != nil {
		return dbErr(err, "ConfirmUTXOs: updating utxos")
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var value string
		err := rows.Scan(&id, &value)
		if err != nil {
			return dbErr(err, "ConfirmUTXOs: scanning row")
		}
		stmt.Exec(id, value)
		if err != nil {
			return dbErr(err, "ConfirmUTXOs: updating account "+id)
		}
	}
	if err = rows.Err(); err != nil { // docs say this check is required!
		return dbErr(err, "ConfirmUTXOs: scanning rows")
	}
	return nil
}

func (t SQLiteStoreTransaction) RevertUTXOsAboveHeight(maxValidHeight int64) error {
	// The presence of a height in adding_height, available_height, spending_height, spent_height
	// indicates that the UTXO is in the process of being added, or has been added (confirmed); is
	// reserved for spending, or has been spent (confirmed)
	_, err := t.tx.Exec("UPDATE utxo SET adding_height=NULL,available_height=NULL,spending_height=NULL,spent_height=NULL WHERE adding_height > ?", maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 1")
	}
	_, err = t.tx.Exec("UPDATE utxo SET available_height=NULL,spending_height=NULL,spent_height=NULL WHERE available_height > ?", maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 2")
	}
	_, err = t.tx.Exec("UPDATE utxo SET spending_height=NULL,spent_height=NULL WHERE spending_height > ?", maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 3")
	}
	_, err = t.tx.Exec("UPDATE utxo SET spent_height=NULL WHERE spent_height > ?", maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertUTXOsAboveHeight: executing update 4")
	}
	return nil
}

func (t SQLiteStoreTransaction) RevertTxnsAboveHeight(maxValidHeight int64) error {
	// The presence of a height in on_chain_height or verified_height indicates
	// that the invoice is in a block (on-chain) or has been verified (N blocks later)
	_, err := t.tx.Exec("UPDATE txn SET on_chain_height = NULL, verified_height = NULL WHERE on_chain_height > ?", maxValidHeight)
	if err != nil {
		return dbErr(err, "RevertTxnsAboveHeight: executing update 1")
	}
	_, err = t.tx.Exec("UPDATE txn SET on_chain_height = NULL, verified_height = NULL WHERE on_chain_height > ?", maxValidHeight)
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
