package giga

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	connect "github.com/dogeorg/dogeconnect-go"
	"github.com/dogeorg/dogeconnect-go/koinu"
	"github.com/shopspring/decimal"
)

const MaxBlockSize = 1000000 // bytes
const MaxTxSize = 10000      // bytes (1/100 of whole block)

var OneDogeDecimal = decimal.NewFromInt(doge.OneDoge)

var (
	ErrNotConnect     = NewErr(BadRequest, "no PaymentRequest issued")
	ErrInvoiceExpired = NewErr(BadRequest, "PaymentRequest has expired")
	ErrInvalidTx      = NewErr(BadRequest, "invalid transaction")
	ErrTxDoesNotPay   = NewErr(BadRequest, "transaction does not pay invoice")
	ErrTxLowFee       = NewErr(BadRequest, "transaction fee too low")
	ErrBadInvoice     = NewErr(BadRequest, "bad invoice")
	ErrSubmitTx       = NewErr(BadRequest, "cannot submit tx")
	ErrNotAvailable   = NewErr(NotAvailable, "service not available")
)

func ConnectPaymentRequest(i Invoice, acc Account, config *Config, rootURL string, privKey []byte) (env connect.ConnectEnvelope, err error) {

	// Sum up totals on the Invoice
	totalAmt := decimal.Zero
	totalTax := decimal.Zero
	totalFee := decimal.Zero
	for _, it := range i.Items {
		value := decimal.NewFromInt(int64(it.Quantity)).Mul(it.Value)
		totalAmt = totalAmt.Add(value)
		if it.Type == "tax" {
			totalTax = totalTax.Add(value)
		} else if it.Type == "fee" {
			totalFee = totalFee.Add(value)
		}
	}

	// Build a DogeConnect Payment Request
	relayURL := fmt.Sprintf("%s/dc/%s", rootURL, i.ID)
	issueTime := i.Created
	r := connect.ConnectPayment{
		Type:          connect.PaymentRequestType,     // MUST be PaymentRequestType
		ID:            string(i.ID),                   // Gateway unique payment-request ID
		Issued:        issueTime.Format(time.RFC3339), // RFC 3339 Timestamp (2006-01-02T15:04:05-07:00)
		Timeout:       config.Connect.PaymentTimeout,  // Seconds; do not submit payment Tx after this time (Issued+Timeout)
		Relay:         relayURL,                       // Payment Relay URL, https://example.com/dc
		FeePerKB:      i.MinFee.String(),              // Minimum fee per 1000 bytes accepted
		MaxSize:       config.Connect.TxMaxSize,       // Maximum tx size in bytes accepted
		VendorIcon:    acc.VendorIcon,                 // vendor icon URL, SHOULD be https:// JPG or PNG
		VendorName:    acc.VendorName,                 // vendor display name
		VendorAddress: acc.VendorAddress,              // vendor business address (optional)
		Total:         totalAmt.String(),              // Total amount including fees and taxes, DECMIAL string
		Fees:          totalFee.String(),              // Fee subtotal, DECMIAL string
		Taxes:         totalTax.String(),              // Taxes subtotal, DECMIAL string
	}
	if !i.FiatTotal.IsZero() {
		r.FiatTotal = i.FiatTotal.String() // Total amount in fiat currency (optional)
		r.FiatCurrency = i.FiatCurrency    // ISO 4217 currency code (required with fiat_total)
	}

	for _, item := range i.Items {
		r.Items = append(r.Items, connect.ConnectItem{
			Type:        "item",
			Icon:        item.ImageLink,
			Name:        item.Name,
			Description: "Description",
			UnitCount:   item.Quantity,
			UnitCost:    item.Value.String(),
			Total:       decimal.NewFromInt(int64(item.Quantity)).Mul(item.Value).String(),
		})
	}

	r.Outputs = append(r.Outputs, connect.ConnectOutput{
		Address: string(i.ID),
		Amount:  totalAmt.String(),
	})

	// Encode and sign the payload
	env, err = connect.SignPaymentRequest(r, privKey[:])
	if err != nil {
		return env, err
	}

	return env, nil
}

func ConnectVerifyTx(invoice Invoice, txBytes []byte, lib L1, store Store, chain *doge.ChainParams) error {
	if invoice.MinFee.IsZero() {
		// No PaymentRequest issued (ConnectPaymentRequest has not been called for this Invoice)
		return ErrNotConnect
	}
	if invoice.Expires.Before(time.Now()) {
		// PaymentRequest has expired (must call ConnectPaymentRequest again)
		return ErrInvoiceExpired
	}

	// decode the transaction
	tx, ok := tryDecodeTx(txBytes)
	if !ok {
		return ErrInvalidTx
	}

	// verify that `tx` pays the invoice
	totalOut, err := verifyInvoiceOutput(tx.VOut, invoice.ID, invoice.Total.String(), chain)
	if err != nil {
		return err
	}

	// verify tx inputs are unspent (and sum all inputs)
	totalIn := int64(0)
	for _, in := range tx.VIn {
		out, err := lib.GetTxOut(hex.EncodeToString(in.TxID), in.VOut, true)
		if err != nil {
			// will be ErrNotFound if the input is spent or doesn't exist
			return ErrInvalidTx
		}
		value, err := koinu.ParseKoinu(out.Value.n)
		if err != nil {
			return ErrInvalidTx
		}
		// sum up all inputs for fee calculation
		totalIn += int64(value)
		if totalIn > koinu.MaxMoney {
			// sum of all inputs exceeds MaxMoney
			return ErrInvalidTx
		}
	}

	// verify fee is reasonable
	numTxBytes := decimal.NewFromInt(int64(len(txBytes)))
	minFee := invoice.MinFee.Mul(numTxBytes)
	txFee := koinuToDoge(totalIn - totalOut)
	if txFee.LessThan(minFee) {
		return ErrTxLowFee
	}

	return nil
}

// koinuToDoge converts from Koinu to a decimal Dogecoin value.
func koinuToDoge(koinu int64) CoinAmount {
	return decimal.NewFromInt(koinu).Shift(-NumKoinuDigits)
}

func tryDecodeTx(txBytes []byte) (tx doge.BlockTx, ok bool) {
	// catch parsing errors.
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	return doge.DecodeTx(txBytes), true
}

func verifyInvoiceOutput(vout []doge.BlockTxOut, payTo doge.Address, amount string, chain *doge.ChainParams) (int64, error) {
	expected, err := koinu.ParseKoinu(amount)
	if err != nil {
		return 0, ErrBadInvoice
	}
	totalOut := int64(0)
	foundPayment := false
	for _, o := range vout {
		// sum up all outputs for fee calculation
		totalOut += o.Value
		if totalOut > koinu.MaxMoney {
			// sum of all outputs exceeds MaxMoney
			return 0, ErrInvalidTx
		}
		// verify the expected amount for Invoice output
		if o.Value == int64(expected) {
			// verify the expected address for Invoice output
			st, addr := doge.ClassifyScript(o.Script, chain)
			if st == doge.ScriptTypeP2PKH && addr == payTo {
				foundPayment = true
			}
		}
	}
	if foundPayment {
		return totalOut, nil
	}
	return 0, ErrTxDoesNotPay
}
