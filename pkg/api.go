package giga

import (
	"fmt"
)

type API struct {
	Store    Store
	L1       L1
	bus      MessageBus
	follower ChainFollower
	config   Config
}

func NewAPI(store Store, l1 L1, bus MessageBus, follower ChainFollower, config Config) API {
	return API{store, l1, bus, follower, config}
}

type InvoiceCreateRequest struct {
	Vendor        string `json:"vendor"`
	Items         []Item `json:"items"`
	Confirmations int32  `json:"confirmations"` // specify -1 to mean not set
}

func (a API) CreateInvoice(request InvoiceCreateRequest, foreignID string) (Invoice, error) {
	txn, err := a.Store.Begin()
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("CreateInvoice: Failed to begin txn: %s", err))
		return Invoice{}, err
	}
	defer txn.Rollback()

	acc, err := txn.GetAccount(foreignID)
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("CreateInvoice: Failed to find Account: %s", foreignID))
		return Invoice{}, err
	}

	// Create a new child address for this invoice from the account's HD key
	keyIndex := acc.NextExternalKey
	invoiceID, err := a.L1.MakeChildAddress(acc.Privkey, keyIndex, false)
	if err != nil {
		eMsg := fmt.Sprintf("MakeChildAddress failed: %v", err)
		a.bus.Send(SYS_ERR, eMsg)
		return Invoice{}, NewErr(UnknownError, eMsg, err)
	}

	confirmations := int32(a.config.Gigawallet.ConfirmationsNeeded)
	if request.Confirmations != -1 {
		confirmations = request.Confirmations
	}

	i := Invoice{ID: invoiceID, Account: acc.Address, Vendor: request.Vendor, Items: request.Items, KeyIndex: keyIndex, Confirmations: confirmations}

	//validate invoice
	err = i.Validate()
	if err != nil {
		return Invoice{}, err
	}

	err = txn.StoreInvoice(i)
	if err != nil {
		return Invoice{}, err
	}

	// Reserve the Invoice Address in the account.
	acc.NextExternalKey = i.KeyIndex + 1
	acc.UpdatePoolAddresses(txn, a.L1)
	txn.UpdateAccount(acc)

	err = txn.Commit()
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("CreateInvoice: Failed to commit: %s", foreignID))
		return Invoice{}, err
	}

	a.bus.Send(INV_CREATED, i)
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
	// Transaction retry loop.
	for {
		txn, err := a.Store.Begin()
		if err != nil {
			a.bus.Send(SYS_ERR, fmt.Sprintf("CreateAccount: Failed to begin txn: %s", err))
			return AccountPublic{}, err
		}
		defer txn.Rollback()

		acc, err := txn.GetAccount(foreignID)
		if err == nil {
			// Account already exists.
			if upsert {
				return acc.GetPublicInfo(), nil
			}
			return AccountPublic{}, NewErr(AlreadyExists, "account already exists: %v", err)
		}

		// Account does not exist yet.
		addr, priv, err := a.L1.MakeAddress(false)
		if err != nil {
			return AccountPublic{}, NewErr(NotAvailable, "cannot create address: %v", err)
		}
		account := Account{
			Address:   addr,
			ForeignID: foreignID,
			Privkey:   priv,
		}

		// Generate and store addresses for transaction discovery on blockchain.
		// This must be done before we store the account!
		err = account.UpdatePoolAddresses(txn, a.L1)
		if err != nil {
			return AccountPublic{}, NewErr(NotAvailable, "cannot generate addresses for account: %v", err)
		}

		// This fails with AlreadyExists if the account exists:
		err = txn.CreateAccount(account)
		if err != nil {
			if IsAlreadyExistsError(err) {
				// retry: another concurrent request created the account.
				txn.Rollback()
				continue
			}
			return AccountPublic{}, NewErr(NotAvailable, "cannot create account: %v", err)
		}

		err = txn.Commit()
		if err != nil {
			a.bus.Send(SYS_ERR, fmt.Sprintf("CreateAccount: Failed to commit: %s", foreignID))
			return AccountPublic{}, NewErr(NotAvailable, "cannot create account: %v", err)
		}

		pub := account.GetPublicInfo()
		a.bus.Send(ACC_CREATED, pub)
		return pub, nil
	}
}

func (a API) GetAccount(foreignID string) (AccountPublic, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return AccountPublic{}, err
	}
	return acc.GetPublicInfo(), nil
}

func (a API) CalculateBalance(foreignID string) (AccountBalance, error) {
	acc, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return AccountBalance{}, err
	}
	bal, err := a.Store.CalculateBalance(acc.Address)
	if err != nil {
		return AccountBalance{}, err
	}
	return bal, nil
}

// Update any of the 'settings' fields on an Account
func (a API) UpdateAccountSettings(foreignID string, update map[string]interface{}) (AccountPublic, error) {
	txn, err := a.Store.Begin()
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("UpdateAccountSettings: Failed to begin txn: %s", err))
		return AccountPublic{}, err
	}
	defer txn.Rollback()

	acc, err := txn.GetAccount(foreignID)
	if err != nil {
		return AccountPublic{}, err
	}
	for k, v := range update {
		switch k {
		case "PayoutAddress":
			acc.PayoutAddress = v.(string)
		case "PayoutThreshold":
			acc.PayoutThreshold = v.(string)
		case "PayoutFrequency":
			acc.PayoutFrequency = v.(string)
		default:
			a.bus.Send(SYS_ERR, fmt.Sprintf("Invalid account setting: %s", k))
		}
	}
	err = txn.UpdateAccount(acc)
	if err != nil {
		return AccountPublic{}, err
	}

	err = txn.Commit()
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("UpdateAccountSettings: Failed to commit: %s", foreignID))
		return AccountPublic{}, err
	}

	pub := acc.GetPublicInfo()
	a.bus.Send(ACC_UPDATED, pub)
	return pub, nil
}

func (a API) SendFundsToAddress(foreignID string, amount CoinAmount, payTo Address) (txid string, fee CoinAmount, err error) {
	account, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return
	}
	if amount.LessThan(TxnDustLimit) {
		return "", ZeroCoins, NewErr(BadRequest, "amount is too small - transaction will be rejected: %s", amount.String())
	}
	builder, err := NewTxnBuilder(&account, a.Store, a.L1)
	if err != nil {
		return
	}
	err = builder.AddUTXOsUpToAmount(amount)
	if err != nil {
		return
	}
	err = builder.AddOutput(payTo, amount)
	if err != nil {
		return
	}
	err = builder.CalculateFee(ZeroCoins)
	if err != nil {
		return
	}
	txn, fee, err := builder.GetFinalTxn()
	if err != nil {
		return
	}

	// Create the Payment record up-front.
	// Save changes to the Account (NextInternalKey) and address pool.
	// Reserve the UTXOs for the payment.
	tx, err := a.Store.Begin()
	if err != nil {
		return
	}
	err = account.UpdatePoolAddresses(tx, a.L1) // we have used an address.
	if err != nil {
		tx.Rollback()
		return
	}
	err = tx.UpdateAccount(account) // for NextInternalKey (change address)
	if err != nil {
		tx.Rollback()
		return
	}
	// TODO: create the `payment` row with NULL block_height & confirm_height & txid
	// payId, err = tx.CreatePayment(account.Address, amount, payTo) // TODO
	// if err != nil {
	//	tx.Rollback()
	// 	return
	// }
	// err = tx.ReserveUTXOsForPayment(payId, builder.GetUTXOs()) // TODO
	// if err != nil {
	//  tx.Rollback()
	// 	return
	// }
	err = tx.Commit()
	if err != nil {
		return
	}

	// Submit the transaction to core.
	txid, err = a.L1.Send(txn.TxnHex)
	if err != nil {
		return
	}

	// Update the Payment with the txid,
	// which changes it to "accepted" status (accepted by the network)
	tx, err = a.Store.Begin()
	if err != nil {
		return
	}
	// err = tx.UpdatePaymentWithTxid(paymentId, txid) // TODO
	// if err != nil {
	//	tx.Rollback()
	// 	return
	// }
	err = tx.Commit()
	if err != nil {
		return
	}

	a.bus.Send(INV_PAYMENT_SENT, map[string]interface{}{"payTo": payTo, "amount": amount, "txid": txid})
	return
}

func (a API) PayInvoiceFromAccount(invoiceID Address, accountID string) (txid string, fee CoinAmount, err error) {
	invoice, err := a.Store.GetInvoice(invoiceID)
	if err != nil {
		return
	}
	account, err := a.Store.GetAccount(accountID)
	if err != nil {
		return
	}
	invoiceAmount := invoice.CalcTotal()
	if invoiceAmount.LessThan(TxnDustLimit) {
		return "", ZeroCoins, fmt.Errorf("invoice amount is too small - transaction will be rejected: %s", invoiceAmount.String())
	}
	payTo := invoice.ID // pay-to Address is the ID

	// Make a Txn to pay `invoiceAmount` from `account` to `payTo`
	builder, err := NewTxnBuilder(&account, a.Store, a.L1)
	if err != nil {
		return
	}
	err = builder.AddUTXOsUpToAmount(invoiceAmount)
	if err != nil {
		return
	}
	err = builder.AddOutput(payTo, invoiceAmount)
	if err != nil {
		return
	}
	err = builder.CalculateFee(ZeroCoins)
	if err != nil {
		return
	}
	txn, fee, err := builder.GetFinalTxn()
	if err != nil {
		return
	}

	// TODO: submit the transaction to Core.
	// TODO: store back the account (to save NextInternalKey; also call UpdatePoolAddresses)
	// TODO: somehow reserve the UTXOs in the interim (prevent accidental double-spend)
	//       until ChainTracker sees the Txn in a Block and calls MarkUTXOSpent.

	a.bus.Send(INV_PAYMENT_SENT, map[string]interface{}{"payTo": payTo, "amount": invoiceAmount})
	return txn.TxnHex, fee, nil
}

// Re-sync from a specific block height, or skip ahead (for now)
func (a API) SetSyncHeight(height int64) error {
	hash, err := a.L1.GetBlockHash(height)
	if err != nil {
		return err
	}
	a.follower.SendCommand(ReSyncChainFollowerCmd{BlockHash: hash})
	return nil
}
