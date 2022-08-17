package giga

type API struct {
	Store Store
	L1    L1
}

func NewAPI(store Store, l1 L1) API {
	return API{store, l1}
}

func (a API) StoreInvoice(invoice Invoice) error {
	return a.Store.StoreInvoice(invoice)
}

func (a API) GetInvoice(id Address) (Invoice, error) {
	return a.Store.GetInvoice(id)
}

// TODO: make this take in the ForeignID instead of the whole Account and return the Address
func (a API) MakeAccount(account Account) error {
	return a.Store.StoreAccount(account)
}

// TODO: make this take in the ForeignID instead of the Address and return the Address
func (a API) GetAccount(id Address) (Account, error) {
	return a.Store.GetAccount(id)
}
