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
	GetPayment(account Address, id int64) (Payment, error)

	// ListPayments returns a list of payments for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListPayments(account Address, cursor int64, limit int) (items []Payment, next_cursor int64, err error)

	// List all unreserved UTXOs in the account's wallet.
	// Unreserved means not already being used in a pending transaction.
	GetAllUnreservedUTXOs(account Address) ([]UTXO, error)

	// GetChainState gets the last saved Best Block information (checkpoint for restart)
	// It returns giga.NotFound if the chainstate record does not exist.
	GetChainState() (ChainState, error)

	// Get a Service Cursor, used to keep track of where services are "up to"
	// in terms of account sequence numbers. This means services can always catch up
	// even if they get a long way behind (e.g. due to a bug, or comms push-back)
	GetServiceCursor(name string) (int64, error)

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

	// Find accounts that have been modified since `cursor` in terms of
	// account sequence numbers. Returns IDs of the accounts and the cursor
	// for the next call (maximum cursor covered by ids, plus one)
	ListAccountsModifiedSince(cursor int64, limit int) (ids []string, nextCursor int64, err error)

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
	CreatePayment(account Address, amount CoinAmount, payTo Address) (Payment, error)

	// GetPayment returns the Payment for the given ID
	GetPayment(account Address, id int64) (Payment, error)

	// Update payment status: txid, paidHeight, notifyHeight.
	// NOTE: always use GetPayment in the same transaction, otherwise this might
	// stomp over changes coming from another thread.
	UpdatePaymentWithTxID(paymentID int64, txID string) error

	// ListPayments returns a list of payments for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListPayments(account Address, cursor int64, limit int) (items []Payment, next_cursor int64, err error)

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

	// UpdateChainState updates the Best Block information (checkpoint for restart)
	UpdateChainState(state ChainState, writeRoot bool) error

	// Create a new Unspent Transaction Output in the database.
	CreateUTXO(utxo UTXO) error

	// Mark an Unspent Transaction Output as spent (storing the given block-height and txid)
	// Returns the ID of the Account that owns this UTXO, if known to Gigawallet.
	MarkUTXOSpent(txID string, vOut int, spentHeight int64, spendTxID string) (accountId string, scriptAddress Address, err error)

	// Mark payments as on-chain that match any of the txIDs, storing the given block-height.
	// Returns the IDs of the Accounts that own any affected payments (can have duplicates)
	MarkPaymentsOnChain(txIDs []string, blockHeight int64) (affectedAcconts []string, err error)

	// Mark all payments as paid after `confirmations` blocks,
	// at the current block height passed in blockHeight. This should be called each
	// time a new block is processed, i.e. blockHeight increases, but it is safe to
	// call less often (e.g. after a batch of blocks)
	ConfirmPayments(confirmations int, blockHeight int64) (affectedAcconts []string, err error)

	// Mark all UTXOs as confirmed (available to spend) after `confirmations` blocks,
	// at the current block height passed in blockHeight. This should be called each
	// time a new block is processed, i.e. blockHeight increases, but it is safe to
	// call less often (e.g. after a batch of blocks)
	ConfirmUTXOs(confirmations int, blockHeight int64) (affectedAcconts []string, err error)

	// Mark all invoices paid that have corresponding confirmed UTXOs [via ConfirmUTXOs]
	// that sum up to the invoice value, storing the given block-height. Returns the IDs
	// of the Accounts that own any affected invoices (can return duplicates)
	MarkInvoicesPaid(blockHeight int64) (affectedAcconts []string, err error)

	// RevertChangesAboveHeight clears chain-heights above the given height recorded in UTXOs and Payments.
	// This serves to roll back the effects of adding or spending those UTXOs and/or Payments.
	RevertChangesAboveHeight(maxValidHeight int64, nextSeq int64) (newSeq int64, err error)

	// Increment the chain-sequence-number for multiple accounts.
	// Use this after modifying accounts' blockchain-derived state (UTXOs, TXNs)
	IncChainSeqForAccounts(accounts map[string]int64) error

	// Update a service cursor (see GetServiceCursor)
	SetServiceCursor(name string, cursor int64) error
}

// Current chainstate in the database.
// Gigawallet TRANSACTIONALLY moves ChainState forward in batches of blocks,
// updating UTXOs, Invoices and Account Balances in the same DB transaction.
type ChainState struct {
	RootHash        string // hash of block at height 1 on the chain being sync'd.
	FirstHeight     int64  // block height when gigawallet first started to sync this blockchain.
	BestBlockHash   string // last block processed by gigawallet (effects included in DB)
	BestBlockHeight int64  // last block height processed by gigawallet (effects included in DB)
	NextSeq         int64  // next sequence-number for services tracking account activity
}

// Get the next sequence-number for tracking account activity.
func (c *ChainState) GetSeq() int64 {
	seq := c.NextSeq
	c.NextSeq += 1
	return seq
}
