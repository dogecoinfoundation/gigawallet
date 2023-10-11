package giga

// UTXO is an Unspent Transaction Output, i.e. a prior payment into our Account.
// This is used in the interface to Store and L1 (libdogecoin)
type UTXO struct {
	TxID          string     // Dogecoin Transaction ID - part of unique key (from Txn Output)
	VOut          int        // Transaction VOut number - part of unique key (from Txn Output)
	Value         CoinAmount // Amount of Dogecoin available to spend (from Txn Output)
	ScriptHex     string     // locking script in this UTXO, hex-encoded
	ScriptType    ScriptType // 'p2pkh' etc, see ScriptType constants (detected from ScriptHex)
	ScriptAddress Address    // P2PKH address required to spend this UTXO (extracted from ScriptHex)
	AccountID     Address    // Account ID (by searching for ScriptAddress using FindAccountForAddress)
	KeyIndex      uint32     // Account HD Wallet key-index of the ScriptAddress (needed to spend)
	IsInternal    bool       // Account HD Wallet internal/external address flag for ScriptAddress (needed to spend)
	BlockHeight   int64      // Block Height of the Block that contains this UTXO (NB. used only when inserting!)
	SpendTxID     string     // TxID of the spending transaction
	PaymentID     int64      // ID of payment in `payment` table (if spent by us)
}
