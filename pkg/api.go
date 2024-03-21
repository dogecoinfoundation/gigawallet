package giga

import (
	"fmt"
	"log"
	"time"

	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
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
	Items         []Item `json:"items"`
	Confirmations int32  `json:"required_confirmations"` // specify -1 to mean not set
}

func (a API) CreateInvoice(request InvoiceCreateRequest, foreignID string) (Invoice, error) {
	dbtx, err := a.Store.Begin()
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("CreateInvoice: Failed to begin txn: %s", err))
		return Invoice{}, err
	}
	defer dbtx.Rollback()

	acc, err := dbtx.GetAccount(foreignID)
	if err != nil {
		a.bus.Send(SYS_ERR, fmt.Sprintf("CreateInvoice: Failed to find Account: %s", foreignID))
		return Invoice{}, err
	}

	// Create a new child address for this invoice from the account's HD key
	invoiceID, keyIndex, err := acc.NextPayToAddress(a.L1)
	if err != nil {
		eMsg := fmt.Sprintf("NextPayToAddress failed: %v", err)
		a.bus.Send(SYS_ERR, eMsg)
		return Invoice{}, NewErr(UnknownError, eMsg, err)
	}

	confirmations := int32(a.config.Gigawallet.ConfirmationsNeeded)
	if request.Confirmations != -1 {
		confirmations = request.Confirmations
	}

	i := Invoice{ID: invoiceID, Account: acc.Address, Items: request.Items, KeyIndex: keyIndex, Confirmations: confirmations, Created: time.Now()}

	//validate invoice
	err = i.Validate()
	if err != nil {
		return Invoice{}, err
	}

	err = dbtx.StoreInvoice(i)
	if err != nil {
		return Invoice{}, err
	}

	// Reserve the Invoice Address in the account.
	err = acc.UpdatePoolAddresses(dbtx, a.L1)
	if err != nil {
		return Invoice{}, err
	}
	err = dbtx.UpdateAccount(acc)
	if err != nil {
		return Invoice{}, err
	}

	err = dbtx.Commit()
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

type AccountCreateRequest struct {
	PayoutAddress   Address    `json:"payout_address"`
	PayoutThreshold CoinAmount `json:"payout_threshold"`
	PayoutFrequency string     `json:"payout_frequency"`
}

func (a API) CreateAccount(request AccountCreateRequest, foreignID string, upsert bool) (AccountPublic, error) {
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
		isTestNet := a.config.Gigawallet.Network == "testnet"
		addr, priv, err := a.L1.MakeAddress(isTestNet)
		if err != nil {
			return AccountPublic{}, NewErr(NotAvailable, "cannot create address: %v", err)
		}
		account := Account{
			Address:         addr,
			ForeignID:       foreignID,
			PayoutAddress:   Address(request.PayoutAddress),
			PayoutThreshold: request.PayoutThreshold,
			PayoutFrequency: request.PayoutFrequency,
			Privkey:         priv,
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
			acc.PayoutAddress = v.(Address)
		case "PayoutThreshold":
			acc.PayoutThreshold, err = doge.ParseCoinAmount(v.(string))
			if err != nil {
				return AccountPublic{}, err
			}
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

type SendFundsResult struct {
	TxId  string     `json:"hex"`
	Total CoinAmount `json:"total"`
	Paid  CoinAmount `json:"paid"`
	Fee   CoinAmount `json:"fee"`
}

func (a API) SendFundsToAddress(foreignID string, payTo []PayTo, explicitFee CoinAmount, maxFee CoinAmount) (res SendFundsResult, err error) {
	account, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return
	}
	if !maxFee.IsPositive() {
		maxFee = TxnRecommendedMaxFee // default maximum fee
	}

	// Create the Dogecoin Transaction
	source := NewUTXOSource(a.Store, account.Address)
	newTxn, err := CreateTxn(payTo, explicitFee, maxFee, account, source, a.L1)
	if err != nil {
		return
	}
	total := newTxn.TotalOut // total paid to addresses (excludes fee)
	fee := newTxn.FeeAmount  // fee paid by the transaction
	txHex := newTxn.TxnHex

	// FIXME: out fee calculation may not round to the nearest Koinu,
	// leading to libdogecoin doing the rounding when it parses the `fee string` argument.
	log.Printf("New Tx: total %v fee %v change %v", newTxn.TotalOut, newTxn.FeeAmount, newTxn.ChangeAmount)

	// Create the Payment record up-front.
	// Save changes to the Account (NextInternalKey) and address pool.
	// Reserve the UTXOs for the payment.
	dbtx, err := a.Store.Begin()
	if err != nil {
		return
	}
	err = account.UpdatePoolAddresses(dbtx, a.L1) // we used a Change address.
	if err != nil {
		dbtx.Rollback()
		return
	}
	err = dbtx.UpdateAccount(account) // for NextInternalKey (change address)
	if err != nil {
		dbtx.Rollback()
		return
	}
	// Create the `payment` row with no txid or paid_height.
	payment, err := dbtx.CreatePayment(account.Address, payTo, total, fee)
	if err != nil {
		dbtx.Rollback()
		return
	}
	// err = tx.ReserveUTXOsForPayment(payId, builder.GetUTXOs()) // TODO
	// if err != nil {
	//  tx.Rollback()
	// 	return
	// }
	err = dbtx.Commit()
	if err != nil {
		return
	}

	// Submit the transaction to core.
	txid, err := a.L1.Send(txHex)
	if err != nil {
		return
	}

	// Update the Payment with the txid,
	// which changes it to "accepted" status (accepted by the network)
	dbtx, err = a.Store.Begin()
	if err != nil {
		return
	}
	err = dbtx.UpdatePaymentWithTxID(payment.ID, txid)
	if err != nil {
		dbtx.Rollback()
		return
	}
	err = dbtx.Commit()
	if err != nil {
		return
	}

	msg := PaymentEvent{
		PaymentID: payment.ID,
		ForeignID: account.ForeignID,
		AccountID: account.Address,
		PayTo:     payTo,
		Total:     total,
		TxID:      txid,
	}
	a.bus.Send(PAYMENT_SENT, msg)

	return SendFundsResult{TxId: txid, Total: total.Add(fee), Paid: total, Fee: fee}, nil
}

func (a API) PayInvoiceFromAccount(invoiceID Address, foreignID string) (res SendFundsResult, err error) {
	invoice, err := a.Store.GetInvoice(invoiceID)
	if err != nil {
		return
	}
	account, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return
	}
	invoiceAmount := invoice.CalcTotal()
	if invoiceAmount.LessThan(TxnDustLimit) {
		return SendFundsResult{}, fmt.Errorf("invoice amount is too small - transaction will be rejected: %s", invoiceAmount.String())
	}
	payToAddress := invoice.ID // pay-to Address is the ID

	// Make a Doge Txn to pay `invoiceAmount` from `account` to `payTo`
	payTo := []PayTo{{PayTo: payToAddress, Amount: invoiceAmount}}
	source := NewUTXOSource(a.Store, account.Address)
	newTxn, err := CreateTxn(payTo, ZeroCoins, TxnRecommendedMaxFee, account, source, a.L1)
	if err != nil {
		return
	}
	fee := newTxn.FeeAmount
	txHex := newTxn.TxnHex

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
	// Create the `payment` row with no txid or paid_height.
	payment, err := tx.CreatePayment(account.Address, payTo, invoiceAmount, fee)
	if err != nil {
		tx.Rollback()
		return
	}
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
	txid, err := a.L1.Send(txHex)
	if err != nil {
		return
	}

	// Update the Payment with the txid,
	// which changes it to "accepted" status (accepted by the network)
	tx, err = a.Store.Begin()
	if err != nil {
		return
	}
	err = tx.UpdatePaymentWithTxID(payment.ID, txid)
	if err != nil {
		tx.Rollback()
		return
	}
	err = tx.Commit()
	if err != nil {
		return
	}

	msg := PaymentEvent{
		PaymentID: payment.ID,
		ForeignID: account.ForeignID,
		AccountID: account.Address,
		PayTo:     payTo,
		Total:     invoiceAmount,
		TxID:      txid,
	}
	a.bus.Send(PAYMENT_SENT, msg)
	return SendFundsResult{TxId: txid, Total: invoiceAmount.Add(fee), Paid: invoiceAmount, Fee: fee}, nil
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
