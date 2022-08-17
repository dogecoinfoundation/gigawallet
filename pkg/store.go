package giga

type Store interface {
	// StoreInvoice stores an invoice.
	StoreInvoice(invoice Invoice) error
	// GetInvoice returns the invoice with the given ID.
	GetInvoice(id Address) (Invoice, error)

	// StoreAccount stores an account.
	StoreAccount(account Account) error
	// GetAccount returns the account with the given ID.
	// TODO: GetAccount by ForeignID
	GetAccount(id Address) (Account, error)
}
