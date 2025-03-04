package giga

import (
	"fmt"
	"time"

	connect "github.com/dogeorg/dogeconnect-go"
	"github.com/shopspring/decimal"
)

func InvoiceToConnectPaymentRequest(i Invoice, rootURL string, privKey []byte) (connect.ConnectEnvelope, error) {

	// Build a DogeConnect Payment Request
	totalAmount := i.CalcTotal().String()
	relayURL := fmt.Sprintf("%s/dc/%s", rootURL, i.ID)
	r := connect.ConnectPayment{
		Type:          connect.PaymentRequestType,      // MUST be PaymentRequestType
		ID:            string(i.ID),                    // Gateway unique payment-request ID
		Issued:        time.Now().Format(time.RFC3339), // RFC 3339 Timestamp (2006-01-02T15:04:05-07:00)
		Timeout:       300,                             // Seconds; do not submit payment Tx after this time (Issued+Timeout)
		Relay:         relayURL,                        // Payment Relay URL, https://example.com/dc
		VendorIcon:    "",                              // vendor icon URL, SHOULD be https:// JPG or PNG
		VendorName:    "",                              // vendor display name
		VendorAddress: "",                              // vendor business address (optional)
		Total:         totalAmount,                     // Total amount including fees and taxes, DECMIAL string
		Fees:          "0.0",                           // Fee subtotal, DECMIAL string
		Taxes:         "0.0",                           // Taxes subtotal, DECMIAL string
		FiatTotal:     "",                              // Total amount in fiat currency (optional)
		FiatCurrency:  "",                              // ISO 4217 currency code (required with fiat_total)
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
		Amount:  totalAmount,
	})

	// Encode and sign the payload
	env, err := connect.SignPaymentRequest(r, privKey[:])
	if err != nil {
		return connect.ConnectEnvelope{}, err
	}

	return env, nil
}
