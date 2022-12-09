package giga

type API struct {
	Store Store
	L1    L1
}

func NewAPI(store Store, l1 L1) API {
	return API{store, l1}
}

type InvoiceCreateRequest struct {
	Vendor string `json:"vendor"`
	Items  []Item `json:"items"`
}

func (a API) CreateInvoice(request InvoiceCreateRequest, foreignID string) (Invoice, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return Invoice{}, NewErr(NotFound, "account not found: %v", foreignID) // XXX assumed
	}
	keyIndex := acc.NextExternalKey
	invoiceID, l1err := a.L1.MakeChildAddress(acc.Privkey, keyIndex, false)
	if l1err != nil {
		return Invoice{}, NewErr(UnknownError, "MakeChildAddress failed: %v", l1err)
	}
	i := Invoice{ID: invoiceID, Account: acc.Address, Vendor: request.Vendor, Items: request.Items, KeyIndex: keyIndex}
	err = a.Store.StoreInvoice(i)
	if err != nil {
		return Invoice{}, NewErr(UnknownError, "StoreInvoice failed: %v", err)
	}
	return i, nil
}

func (a API) GetInvoice(id Address) (Invoice, error) {
	inv, err := a.Store.GetInvoice(id)
	if err != nil {
		return Invoice{}, NewErr(UnknownError, "GetInvoice failed: %v", err)
	}
	return inv, nil
}

type ListInvoicesResponse struct {
	Items  []Invoice `json:"items"`
	Cursor int       `json:"cursor"`
}

func (a API) ListInvoices(foreignID string, cursor int, limit int) (ListInvoicesResponse, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return ListInvoicesResponse{}, NewErr(NotFound, "account not found: %v", foreignID) // XXX assumed
	}
	items, next_cursor, err := a.Store.ListInvoices(acc.Address, cursor, limit)
	if err != nil {
		return ListInvoicesResponse{}, NewErr(UnknownError, "ListInvoices failed: %v", err)
	}
	if items == nil {
		items = []Invoice{} // encoded as '[]' in JSON
	}
	r := ListInvoicesResponse{
		Items:  items,
		Cursor: next_cursor,
	}
	return r, nil
}

func (a API) CreateAccount(foreignID string, upsert bool) (AccountPublic, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err == nil {
		if upsert {
			return acc.GetPublicInfo(), nil
		}
		return AccountPublic{}, NewErr(AlreadyExists, "account already exists: %v", foreignID)
	}
	addr, priv, err := a.L1.MakeAddress()
	if err != nil {
		return AccountPublic{}, NewErr(UnknownError, "MakeAddress failed: %v", err)
	}
	account := Account{
		Address:   addr,
		ForeignID: foreignID,
		Privkey:   priv,
	}
	err = a.Store.StoreAccount(account)
	if err != nil {
		return AccountPublic{}, NewErr(UnknownError, "StoreAccount failed: %v", err)
	}
	return account.GetPublicInfo(), nil
}

func (a API) GetAccount(foreignID string) (AccountPublic, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return AccountPublic{}, NewErr(NotFound, "account not found: %v", foreignID) // XXX assumed
	}
	return acc.GetPublicInfo(), nil
}
