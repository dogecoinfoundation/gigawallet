package giga

// A store represents a connection to a database
// with a transactional API that
type Store interface {
	Begin() (StoreTransaction, error)

	// GetAccount returns the account with the given ForeignID.
	GetAccount(foreignID string) (Account, error)

	// CalculateBalance queries across UTXOs to calculate account balances.
	CalculateBalance(accountID Address) (AccountBalance, error)

	// GetInvoice returns the invoice with the given ID.
	GetInvoice(id Address) (Invoice, error)

	// ListInvoices returns a filtered list of invoices for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListInvoices(account Address, cursor int, limit int) (items []Invoice, next_cursor int, err error)

	// GetPayment returns the Payment for the given ID
	GetPayment(id int) (Payment, error)

	// ListPayments returns a list of payments for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListPayments(account Address, cursor int, limit int) (items []Payment, next_cursor int, err error)

	// List all unreserved UTXOs in the account's wallet.
	// Unreserved means not already being used in a pending transaction.
	GetAllUnreservedUTXOs(account Address) ([]UTXO, error)

	// GetChainState gets the last saved Best Block information (checkpoint for restart)
	// It returns giga.NotFound if the chainstate record does not exist.
	GetChainState() (ChainState, error)

	// Close the store.
	Close()
}

type StoreTransaction interface {
	// Commit the transaction to the store
	Commit() error
	// Rollback the transaction from the store, should
	// be a no-op of Commit has already succeeded
	Rollback() error

	// GetAccount returns the account with the given ForeignID.
	// It returns giga.NotFound if the account does not exist (key: ForeignID)
	GetAccount(foreignID string) (Account, error)

	// GetAccount returns the account with the given ID.
	// It returns giga.NotFound if the account does not exist (key: ID)
	GetAccountByID(ID string) (Account, error)

	// CalculateBalance queries across UTXOs to calculate account balances.
	CalculateBalance(accountID Address) (AccountBalance, error)

	// GetInvoice returns the invoice with the given ID.
	// It returns giga.NotFound if the invoice does not exist (key: ID/address)
	GetInvoice(id Address) (Invoice, error)

	// ListInvoices returns a filtered list of invoices for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListInvoices(account Address, cursor int, limit int) (items []Invoice, next_cursor int, err error)

	// List all unreserved UTXOs in the account's wallet.
	// Unreserved means not already being used in a pending transaction.
	GetAllUnreservedUTXOs(account Address) ([]UTXO, error)

	// StoreInvoice stores an invoice.
	// Caller SHOULD update Account.NextExternalKey and use StoreAccount in the same StoreTransaction.
	// It returns an unspecified error if the invoice ID already exists (FIXME)
	StoreInvoice(invoice Invoice) error

	// Store a 'payment' which represents a pay-out to another address from a gigawallet
	// managed account.
	CreatePayment(Address, CoinAmount, Address) (Payment, error)

	// GetPayment returns the Payment for the given ID
	GetPayment(id int) (Payment, error)

	// ListPayments returns a list of payments for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListPayments(account Address, cursor int, limit int) (items []Payment, next_cursor int, err error)

	// CreateAccount stores a NEW account.
	// It returns giga.AlreadyExists if the account already exists (key: ForeignID)
	CreateAccount(account Account) error

	// UpdateAccount updates an existing account.
	// It returns giga.NotFound if the account does not exist (key: ForeignID)
	// NOTE: will not update 'Privkey' or 'Address' (changes ignored or rejected)
	// NOTE: counters can only be advanced, not regressed (e.g. NextExternalKey) (ignored or rejected)
	UpdateAccount(account Account) error

	// StoreAddresses associates a list of addresses with an accountID
	StoreAddresses(accountID Address, addresses []Address, firstAddress uint32, internal bool) error

	// Find the accountID (HD root PKH) that owns the given Dogecoin address.
	// Also find the key index of `pkhAddress` within the HD wallet.
	FindAccountForAddress(pkhAddress Address) (accountID Address, keyIndex uint32, isInternal bool, err error)

	// What it says on the tin. We should consider
	// adding this to Store as a fast-path
	MarkInvoiceAsPaid(address Address) error

	// UpdateChainState updates the Best Block information (checkpoint for restart)
	UpdateChainState(state ChainState, writeRoot bool) error

	// Create a new Unspent Transaction Output in the database.
	CreateUTXO(utxo UTXO) error

	// Mark an Unspent Transaction Output as spent (at the given block height)
	// Returns the ID of the Account that can spend this UTXO, if known to Gigawallet.
	MarkUTXOSpent(txID string, vOut int, spentHeight int64) (accountId string, scriptAddress Address, err error)

	// Mark all UTXOs as confirmed (available to spend) after `confirmations` blocks,
	// at the current block height passed in blockHeight. This should be called each
	// time a new block is processed, i.e. blockHeight increases, but it is safe to
	// call less often (e.g. after a batch of blocks)
	ConfirmUTXOs(confirmations int, blockHeight int64) (affectedAcconts []string, err error)

	// RevertUTXOsAboveHeight clears chain-heights above the given height recorded in UTXOs.
	// This serves to roll back the effects of adding or spending those UTXOs.
	RevertUTXOsAboveHeight(maxValidHeight int64) error

	// RevertTxnsAboveHeight clears chain-heights above the given height recorded in Txns.
	// This serves to roll back the effects of creating or confirming those Txns.
	RevertTxnsAboveHeight(maxValidHeight int64) error

	// Increment the chain-sequence-number for multiple accounts.
	// Use this after modifying accounts' blockchain-derived state (UTXOs, TXNs)
	IncChainSeqForAccounts(accountIds []string) error

	// Find all accounts with UTXOs or TXNs created or modified above the specified block height,
	// and increment those accounts' chain-sequence-number.
	// MUST be done before rolling back chainstate, i.e. RevertUTXOsAboveHeight, RevertTxnsAboveHeight.
	IncAccountsAffectedByRollback(maxValidHeight int64) ([]string, error)
}

// Current chainstate in the database.
// Gigawallet TRANSACTIONALLY moves ChainState forward in batches of blocks,
// updating UTXOs, Invoices and Account Balances in the same DB transaction.
type ChainState struct {
	RootHash        string // hash of block at height 1 on the chain being sync'd.
	FirstHeight     int64  // block height when gigawallet first started to sync this blockchain.
	BestBlockHash   string // last block processed by gigawallet (effects included in DB)
	BestBlockHeight int64  // last block height processed by gigawallet (effects included in DB)
}
