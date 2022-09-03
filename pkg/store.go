package giga

type Store interface {
	// StoreInvoice stores an invoice.
	StoreInvoice(invoice Invoice) error
	// GetInvoice returns the invoice with the given ID.
	GetInvoice(id Address) (Invoice, error)
	// GetPendingInvoices sends all invoices that are pending to the given channel.
	GetPendingInvoices() (<-chan Invoice, error)
	// MarkInvoiceAsPaid marks the invoice with the given ID as paid.
	MarkInvoiceAsPaid(id Address) error

	// StoreAccount stores an account.
	StoreAccount(account Account) error
	// GetAccount returns the account with the given ForeignID.
	GetAccount(foreignID string) (Account, error)
	// GetAccountByAddress returns the account with the given Address.
	GetAccountByAddress(id Address) (Account, error)
}
