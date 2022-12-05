package giga

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

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
		return Invoice{}, err
	}
	keyIndex := acc.NextExternalKey
	invoiceID, err := a.L1.MakeChildAddress(acc.Privkey, keyIndex, false)
	if err != nil {
		return Invoice{}, err
	}
	// generate an access token
	b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
		return Invoice{}, err
	}
	token := base64.URLEncoding.EncodeToString(b)

	i := Invoice{ID: invoiceID, Account: acc.Address, Vendor: request.Vendor, Items: request.Items, AccessToken: token, KeyIndex: keyIndex}
	err = a.Store.StoreInvoice(i)
	if err != nil {
		return Invoice{}, err
	}
	return i, nil
}

func (a API) GetInvoice(id Address) (Invoice, error) {
	return a.Store.GetInvoice(id)
}

func (a API) CreateAccount(foreignID string) (Address, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err == nil {
		return "", errors.New("account already exists with address " + string(acc.Address))
	}
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

func (a API) GetAccountByAddress(id Address) (AccountPublic, error) {
	acc, err := a.Store.GetAccountByAddress(id)
	if err != nil {
		return AccountPublic{}, err
	}
	return acc.GetPublicInfo(), nil
}
