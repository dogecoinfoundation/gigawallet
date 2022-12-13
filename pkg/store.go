package giga

type Store interface {
	// StoreInvoice stores an invoice.
	StoreInvoice(invoice Invoice) error
	// GetInvoice returns the invoice with the given ID.
	GetInvoice(id Address) (Invoice, error)
	// ListInvoices returns a filtered list of invoices for an account.
	// pagination: next_cursor should be passed as 'cursor' on the next call (initial cursor = 0)
	// pagination: when next_cursor == 0, that is the final page of results.
	// pagination: stores CAN return < limit (or zero) items WITH next_cursor > 0 (due to filtering)
	ListInvoices(account Address, cursor int, limit int) (items []Invoice, next_cursor int, err error)
	// GetPendingInvoices sends all invoices that are pending to the given channel.
	GetPendingInvoices() (<-chan Invoice, error)
	// MarkInvoiceAsPaid marks the invoice with the given ID as paid.
	MarkInvoiceAsPaid(id Address) error

	// StoreAccount stores an account.
	StoreAccount(account Account) error
	// GetAccount returns the account with the given ForeignID.
	GetAccount(foreignID string) (Account, error)

	// List all unreserved UTXOs in the account's wallet.
	// Unreserved means not already being used in a pending transaction.
	GetAllUnreservedUTXOs(account Address) ([]UTXO, error)
}
