package giga

import (
	"fmt"
)

type API struct {
	Store Store
	L1    L1
}

func NewAPI(store Store, l1 L1) API {
	return API{store, l1}
}

type APIErrorCode string

const (
	NotFound      APIErrorCode = "not-found"
	AlreadyExists APIErrorCode = "already-exists"
	UnknownError  APIErrorCode = "unknown-error"
)

type APIError struct {
	Code    APIErrorCode `json:"code"`
	Message string       `json:"message"`
}

func fmtErr(code APIErrorCode, format string, args ...any) *APIError {
	return &APIError{Code: code, Message: fmt.Sprintf(format, args...)}
}

type InvoiceCreateRequest struct {
	Vendor string `json:"vendor"`
	Items  []Item `json:"items"`
}

func (a API) CreateInvoice(request InvoiceCreateRequest, foreignID string) (Invoice, *APIError) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return Invoice{}, fmtErr(NotFound, "account not found: %v", foreignID) // XXX assumed
	}
	keyIndex := acc.NextExternalKey
	invoiceID, err := a.L1.MakeChildAddress(acc.Privkey, keyIndex, false)
	if err != nil {
		return Invoice{}, fmtErr(UnknownError, "MakeChildAddress failed: %v", err)
	}
	i := Invoice{ID: invoiceID, Account: acc.Address, Vendor: request.Vendor, Items: request.Items, KeyIndex: keyIndex}
	err = a.Store.StoreInvoice(i)
	if err != nil {
		return Invoice{}, fmtErr(UnknownError, "StoreInvoice failed: %v", err)
	}
	return i, nil
}

func (a API) GetInvoice(id Address) (Invoice, *APIError) {
	inv, err := a.Store.GetInvoice(id)
	if err != nil {
		return Invoice{}, fmtErr(UnknownError, "GetInvoice failed: %v", err)
	}
	return inv, nil
}

type ListInvoicesResponse struct {
	Items  []Invoice `json:"items"`
	Cursor int       `json:"cursor"`
}

func (a API) ListInvoices(foreignID string, cursor int, limit int) (ListInvoicesResponse, *APIError) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return ListInvoicesResponse{}, fmtErr(NotFound, "account not found: %v", foreignID) // XXX assumed
	}
	items, next_cursor, err := a.Store.ListInvoices(acc.Address, cursor, limit)
	if err != nil {
		return ListInvoicesResponse{}, fmtErr(UnknownError, "ListInvoices failed: %v", err)
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

func (a API) CreateAccount(foreignID string, upsert bool) (AccountPublic, *APIError) {
	acc, err := a.Store.GetAccount(foreignID)
	if err == nil {
		if upsert {
			return acc.GetPublicInfo(), nil
		}
		return AccountPublic{}, fmtErr(AlreadyExists, "account already exists: %v", foreignID)
	}
	addr, priv, err := a.L1.MakeAddress()
	if err != nil {
		return AccountPublic{}, fmtErr(UnknownError, "MakeAddress failed: %v", err)
	}
	account := Account{
		Address:   addr,
		ForeignID: foreignID,
		Privkey:   priv,
	}
	err = a.Store.StoreAccount(account)
	if err != nil {
		return AccountPublic{}, fmtErr(UnknownError, "StoreAccount failed: %v", err)
	}
	return account.GetPublicInfo(), nil
}

func (a API) GetAccount(foreignID string) (AccountPublic, *APIError) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return AccountPublic{}, fmtErr(NotFound, "account not found: %v", foreignID) // XXX assumed
	}
	return acc.GetPublicInfo(), nil
}

func (a API) GetAccountByAddress(id Address) (AccountPublic, *APIError) {
	acc, err := a.Store.GetAccountByAddress(id)
	if err != nil {
		return AccountPublic{}, fmtErr(NotFound, "account not found: %v", id) // XXX assumed
	}
	return acc.GetPublicInfo(), nil
}
