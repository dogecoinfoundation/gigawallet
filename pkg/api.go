package giga

import (
	"fmt"
)

const (
	scriptTypeP2PKH    = "p2pkh"
	scriptTypeMultiSig = "multisig"
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
		return Invoice{}, NewErr(UnknownError, "MakeChildAddress failed: %v", err)
	}
	i := Invoice{ID: invoiceID, Account: acc.Address, Vendor: request.Vendor, Items: request.Items, KeyIndex: keyIndex}
	err = a.Store.StoreInvoice(i)
	if err != nil {
		return Invoice{}, err
	}
	return i, nil
}

func (a API) GetInvoice(id Address) (Invoice, error) {
	inv, err := a.Store.GetInvoice(id)
	if err != nil {
		return Invoice{}, err
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
		return ListInvoicesResponse{}, err
	}
	items, next_cursor, err := a.Store.ListInvoices(acc.Address, cursor, limit)
	if err != nil {
		return ListInvoicesResponse{}, err
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
	fmt.Println("create account")
	acc, err := a.Store.GetAccount(foreignID)
	if err == nil {
		if upsert {
			return acc.GetPublicInfo(), nil
		}
		return AccountPublic{}, err
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
	fmt.Println("store account", account)
	acc, err = a.Store.GetAccount(foreignID)
	err = a.Store.StoreAccount(account)
	if err != nil {
		return AccountPublic{}, err
	}
	return account.GetPublicInfo(), nil
}

func (a API) GetAccount(foreignID string) (AccountPublic, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return AccountPublic{}, err
	}
	return acc.GetPublicInfo(), nil
}

func (a API) PayInvoiceFromAccount(invoiceID Address, accountID string) (string, error) {
	invoice, err := a.Store.GetInvoice(invoiceID)
	if err != nil {
		return "", err
	}
	payFrom, err := a.Store.GetAccount(accountID)
	if err != nil {
		return "", err
	}
	// chicken and egg problem: fee calculation requires transaction size,
	// and transaction size depends on the number of UTXOs included...
	// start with enough UTXOs to pay for at least 1 KB (1000 bytes) and,
	// if the txn turns out bigger than the selected UTXOs can pay for,
	// we'll add another UTXO and try again (note that we select enough
	// UTXOs to pay for _at_least_ 1000 bytes, but may cover a lot more)
	invoiceAmount := invoice.CalcTotal()
	if invoiceAmount.LessThan(TxnDustLimit) {
		return "", fmt.Errorf("invoice amount is too small - transaction will be rejected: %s", invoiceAmount.String())
	}
	feeGuess := TxnFeePerKB // TODO: use transaction size.
	amountPlusFee := invoiceAmount.Add(feeGuess)
	unspentUTXOs, err := payFrom.UnreservedUTXOs(a.Store)
	if err != nil {
		return "", err
	}
	txnInputs := chooseUTXOsToSpend(amountPlusFee, unspentUTXOs)
	if txnInputs == nil {
		return "", fmt.Errorf("insufficient funds in account: %s", accountID)
	}
	payTo := invoice.ID // pay-to Address is the ID
	changeAddress, err := payFrom.NextChangeAddress(a.L1)
	if err != nil {
		return "", err
	}
	// create a transaction to pay the invoice amount (plus fee)
	// from the `payFrom` account, paying any change back to the `payFrom` account
	txn, err := a.L1.MakeTransaction(invoiceAmount, txnInputs, payTo, feeGuess, changeAddress, payFrom.Privkey)
	if err != nil {
		return "", err
	}
	// TODO: adjust the fee based on txn size and make the Txn again?
	// TODO: mark the chosen UTXOs as being spent by this Txn (which we must also track in the pay-from account)
	// TODO: mark the change address as being used by this Txn (in the 'from' account)
	// TODO: mark the pay-to address as being used by this Txn (in the 'to' account)
	// TODO: submit the transaction to the mempool
	// TODO: mark the transaction as 'in progress' in the DB (must affect both accounts)
	return txn.TxnHex, nil
}

// chooseUTXOsToSpend selects unspent UTXOs from the Account (Wallet)
// that add up to some specified amount, [TODO: plus the transaction fees
// for the UTXO inputs added by this function, which change the size of
// the transaction as we add them.]
// The UTXOIterator is assumed to be sorted in the order we want to
// spend them by the Account/Wallet itself.
func chooseUTXOsToSpend(minimumTotal CoinAmount, unspentUTXOs UTXOIterator) (selected []UTXO) {
	remaining := minimumTotal
	for unspentUTXOs.hasNext() {
		utxo := unspentUTXOs.getNext()
		if utxo.ScriptType == scriptTypeP2PKH {
			// we can (presumably) spend this UTXO with one of our private keys,
			// otherwise it wouldn't be in our wallet.
			// XXX: should grow the minimumTotal by the post-signed UTXO "input" size
			// each time we add a UTXO to the transaction here.
			if utxo.Value.GreaterThanOrEqual(remaining) {
				selected = append(selected, utxo)
				return // up to and including this UTXO
			} else {
				remaining = remaining.Sub(utxo.Value)
			}
		}
	}
	return
}
