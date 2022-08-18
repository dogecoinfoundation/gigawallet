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

func (a API) MakeAccount(foreignID string) (Address, error) {
	addr, priv, err := a.L1.MakeAddress()
	if err != nil {
		return "", err
	}
	account := Account{
		Address:   addr,
		ForeignID: foreignID,
		Privkey:   priv,
	}
	return account.Address, a.Store.StoreAccount(account)
}

func (a API) GetAccount(foreignID string) (AccountPublic, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return AccountPublic{}, err
	}
	return acc.GetPublicInfo(), nil
}

func (a API) GetAccountByAddress(id Address) (Account, error) {
	return a.Store.GetAccountByAddress(id)
}
