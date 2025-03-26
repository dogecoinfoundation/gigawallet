package giga

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	connect "github.com/dogeorg/dogeconnect-go"
	"github.com/shopspring/decimal"
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
	err = dbtx.UpdateAccountKeys(acc)
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

func (a API) GetInvoiceConnectURL(invoice Invoice, rootURL string) (string, error) {
	// Get the Account by its internal ID, rather than by foreign id.
	tx, err := a.Store.Begin()
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	acc, err := tx.GetAccountByID(invoice.Account)
	if err != nil {
		return "", fmt.Errorf("bad account: %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	// The DogeConnect Envelope is signed using the account key.
	// The key in the account is an Extended BIP32 Privkey.
	privKey, err := doge.DecodeBip32WIF(string(acc.Privkey), nil)
	if err != nil {
		return "", fmt.Errorf("bad key in account: %w", err)
	}
	defer privKey.Clear()
	pubBytes := privKey.GetECPubKey()[1:] // X-only Public Key
	defer clear(pubBytes)

	connectURL := fmt.Sprintf("%s/dc/%s", rootURL, invoice.ID)
	uri := connect.DogecoinURI(string(invoice.ID), invoice.CalcTotal().String(), connectURL, pubBytes)

	return uri, nil
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

func (a API) CreateAccount(request map[string]any, foreignID string, upsert bool) (AccountPublic, error) {
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
			// Account already exists, apply updates.
			if upsert {
				acc, err := a.applyAccountSettings(acc, request)
				if err != nil {
					return AccountPublic{}, NewErr(BadRequest, err.Error())
				}
				err = txn.UpdateAccountConfig(acc)
				if err != nil {
					return AccountPublic{}, NewErr(NotAvailable, "cannot update account: %v", err)
				}
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
			Address:   addr,
			ForeignID: foreignID,
			Privkey:   priv,
		}
		account, err = a.applyAccountSettings(account, request)
		if err != nil {
			return AccountPublic{}, NewErr(BadRequest, err.Error())
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
func (a API) applyAccountSettings(acc Account, update map[string]any) (Account, error) {
	var err error
	for k, v := range update {
		if val, ok := v.(string); ok {
			switch k {
			case "payout_address":
				if !doge.ValidateP2PKH(Address(val), &doge.DogeMainNetChain) {
					return acc, fmt.Errorf("invalid main-net address: %v", val)
				}
				acc.PayoutAddress = Address(val)
			case "payout_threshold":
				acc.PayoutThreshold, err = decimal.NewFromString(val)
				if err != nil {
					return acc, err
				}
			case "payout_frequency":
				acc.PayoutFrequency = val // format not yet specified
			case "vendor_name":
				acc.VendorName = val
			case "vendor_icon":
				acc.VendorIcon = val
			case "vendor_address":
				acc.VendorAddress = val
			default:
				return acc, fmt.Errorf("invalid account setting '%s'", k)
			}
		}
	}
	return acc, nil
}

type SendFundsResult struct {
	TxId   string     `json:"hex"`   // hash of the transaction (must be unique on-chain)
	Total  CoinAmount `json:"total"` // total amount spent, including fee
	Paid   CoinAmount `json:"paid"`  // amount paid to recipient, excluding fee
	Fee    CoinAmount `json:"fee"`   // fee paid
	TxData string     `json:"tx"`    // transaction data, hex-encoded
}

func (a API) SendFundsToAddress(foreignID string, payTo []PayTo, explicitFee CoinAmount, maxFee CoinAmount, sendTx bool) (res SendFundsResult, err error) {
	account, err := a.Store.GetAccount(foreignID)
	if err != nil {
		return
	}
	if !maxFee.IsPositive() {
		maxFee = TxnRecommendedMaxFee // default maximum fee
	}

	// Create the Dogecoin Transaction
	source := NewUTXOSource(a.Store, account.Address)
	newTxn, changeUTXO, spentUTXOs, txid, err := CreateTxn(payTo, explicitFee, maxFee, account, source, a.L1)
	if err != nil {
		return
	}
	total := newTxn.TotalOut // total paid to addresses (excludes fee)
	fee := newTxn.FeeAmount  // fee paid by the transaction
	txHex := newTxn.TxnHex

	log.Printf("New Tx: total %v fee %v change %v", newTxn.TotalOut, newTxn.FeeAmount, newTxn.ChangeAmount)

	// Create the Payment record up-front.
	// Save changes to the Account (NextInternalKey) and address pool.
	// Reserve the UTXOs for the payment.
	dbtx, err := a.Store.Begin()
	if err != nil {
		return
	}
	defer dbtx.Rollback()
	err = account.UpdatePoolAddresses(dbtx, a.L1) // we used a Change address.
	if err != nil {
		dbtx.Rollback()
		return
	}
	err = dbtx.UpdateAccountKeys(account) // for NextInternalKey (change address)
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
	// Reserve the UTXOs we're spending so they can't be double-spent.
	for _, utxo := range spentUTXOs {
		err = dbtx.MarkUTXOReserved(utxo.TxID, utxo.VOut, payment.ID)
		if err != nil {
			dbtx.Rollback()
			return
		}
	}
	// Create the 'change' UTXO now, so the change can be spent immediately.
	if !changeUTXO.Value.IsZero() {
		err = dbtx.CreateUTXO(changeUTXO)
		if err != nil {
			dbtx.Rollback()
			return
		}
	}
	err = dbtx.Commit()
	if err != nil {
		return
	}

	// BEYOND THIS POINT: if we fail to submit the tx, user must void the payment
	// manually which will clear the reserved lock on the UTXOs being spent.

	// Submit the transaction to core.
	if sendTx {
		// Submit tx to the network.
		coreTxid, e := a.L1.Send(txHex)
		if e != nil {
			err = e
			return
		}
		if coreTxid != txid {
			log.Printf("[!] sendrawtransaction: Core Node did not return the precomputed txid: %s (expecting %s)", coreTxid, txid)
		}
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

	if sendTx {
		msg := PaymentEvent{
			PaymentID: payment.ID,
			ForeignID: account.ForeignID,
			AccountID: account.Address,
			PayTo:     payTo,
			Total:     total,
			TxID:      txid,
		}
		a.bus.Send(PAYMENT_SENT, msg)
	}

	return SendFundsResult{TxId: txid, Total: total.Add(fee), Paid: total, Fee: fee, TxData: txHex}, nil
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
	newTxn, changeUTXO, spentUTXOs, txid, err := CreateTxn(payTo, ZeroCoins, TxnRecommendedMaxFee, account, source, a.L1)
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
	defer tx.Rollback()
	err = account.UpdatePoolAddresses(tx, a.L1) // we have used an address.
	if err != nil {
		tx.Rollback()
		return
	}
	err = tx.UpdateAccountKeys(account) // for NextInternalKey (change address)
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
	// Reserve the UTXOs we're spending so they can't be double-spent.
	for _, utxo := range spentUTXOs {
		err = tx.MarkUTXOReserved(utxo.TxID, utxo.VOut, payment.ID)
		if err != nil {
			tx.Rollback()
			return
		}
	}
	// Create the 'change' UTXO now, so the change can be spent immediately.
	if !changeUTXO.Value.IsZero() {
		err = tx.CreateUTXO(changeUTXO)
		if err != nil {
			tx.Rollback()
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		return
	}

	// Submit the transaction to core.
	coreTxid, err := a.L1.Send(txHex)
	if err != nil {
		return
	}
	if coreTxid != txid {
		log.Printf("[!] sendrawtransaction: Core Node did not return the precomputed txid: %s (expecting %s)", coreTxid, txid)
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

func (a API) GetInvoiceConnectEnvelope(invoice Invoice, rootURL string) (env connect.ConnectEnvelope, err error) {
	// Get the Account by its internal ID, rather than by foreign id.
	var acc Account
	a.Store.Transact(func(tx StoreTransaction) error {
		acc, err = tx.GetAccountByID(invoice.Account)
		return err
	})

	// Sign the DogeConnect Envelope using the account key.
	// The key in the account is an Extended BIP32 Privkey.
	privKey, err := doge.DecodeBip32WIF(string(acc.Privkey), nil)
	if err != nil {
		return env, fmt.Errorf("bad key in account: %w", err)
	}
	defer privKey.Clear()
	privBytes, err := privKey.GetECPrivKey()
	if err != nil {
		return env, fmt.Errorf("bad key in account: %w", err)
	}
	defer clear(privBytes)

	// Create and sign the Payment Request.
	env, minFee, expires, err := ConnectPaymentRequest(invoice, acc, a.L1, &a.config, rootURL, privBytes)
	if err != nil {
		return env, fmt.Errorf("signing envelope: %w", err)
	}

	// Save minFee on the Invoice for verification in PayConnectInvoice
	a.Store.Transact(func(tx StoreTransaction) error {
		return tx.SetInvoiceConnect(invoice.ID, minFee, expires)
	})

	return env, nil
}

func (a API) PayConnectInvoice(invoice Invoice, tx string) error {
	txBytes, err := hex.DecodeString(tx)
	if err != nil {
		return ErrInvalidTx
	}

	// verify the submitted `tx`
	err = ConnectVerifyTx(invoice, txBytes, a.L1, a.Store, a.config.Chain)
	if err != nil {
		return err
	}

	// record the submitted `tx` on the Invoice
	a.Store.Transact(func(tx StoreTransaction) error {
		return tx.SetInvoiceTx(invoice.ID, txBytes)
	})

	// submit the tx to core
	_, err = a.L1.Send(tx)
	if err != nil {
		return ErrSubmitTx
	}

	return nil
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
