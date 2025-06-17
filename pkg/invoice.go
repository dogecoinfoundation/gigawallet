package giga

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Invoice is a request for payment created by Gigawallet.
type Invoice struct {
	// ID is the single-use address that the invoice needs to be paid to.
	ID            Address    `json:"id"`      // pay-to Address (Invoice ID)
	Account       Address    `json:"account"` // an Account.Address (Account ID)
	Items         []Item     `json:"items"`
	Confirmations int32      `json:"required_confirmations"` // number of confirmed blocks (since block_id)
	Created       time.Time  `json:"created"`
	Total         CoinAmount `json:"total"` // derived from items
	// These are used internally to track invoice status.
	KeyIndex           uint32     `json:"-"`               // which HD Wallet child-key was generated
	BlockID            string     `json:"-"`               // transaction seen in this mined block
	PaidHeight         int64      `json:"-"`               // block-height when the invoice was marked as paid
	PaidEvent          time.Time  `json:"-"`               // timestamp when INV_PAID event was sent
	IncomingAmount     CoinAmount `json:"total_incoming"`  // total of all incoming UTXOs
	PaidAmount         CoinAmount `json:"total_confirmed"` // total of all confirmed UTXOs
	LastIncomingAmount CoinAmount `json:"-"`               // last incoming total used to send an event
	LastPaidAmount     CoinAmount `json:"-"`               // last confirmed total used to send an event
	// Additional derived fields (included in PublicInvoice)
	PayTo          Address `json:"pay_to_address"`
	PartDetected   bool    `json:"part_payment_detected"`       // Calculated
	TotalDetected  bool    `json:"total_payment_detected"`      // Calculated
	TotalConfirmed bool    `json:"total_payment_confirmed"`     // Calculated
	Unconfirmed    bool    `json:"payment_unconfirmed"`         // Calculated
	Estimate       int     `json:"estimate_seconds_to_confirm"` // Calculated
}

// CalcTotal sums up the Items listed on the Invoice.
func (i *Invoice) CalcTotal() CoinAmount {
	total := ZeroCoins
	for _, item := range i.Items {
		total = total.Add(decimal.NewFromInt(int64(item.Quantity)).Mul(item.Value))
	}
	return total
}

// Various types of line item in an invoice
var ItemTypes []string = []string{
	"item",     // general purpose line item
	"tax",      // some form of tax
	"fee",      // any fee applied to the invoice
	"shipping", // shipping cost
	"discount", // a discount, a negative number
	"donation", // a donation for some cause
}

type Item struct {
	Type        string     `json:"type"` //ItemTypes
	Name        string     `json:"name"`
	SKU         string     `json:"sku"`
	Description string     `json:"description"`
	Value       CoinAmount `json:"value"`
	Quantity    int        `json:"quantity"`
	ImageLink   string     `json:"image_link"`
}

func (i *Invoice) Validate() error {
	// Has items
	if len(i.Items) == 0 {
		return errors.New("Invoice contains no items")
	}

	// Validate each item
	for _, item := range i.Items {
		// Quantity should be greater than zero
		if item.Quantity <= 0 {
			return errors.New("Item quantity should be greater than zero")
		}

		// Value should be greater than zero, unless type is discount
		if item.Type == "discount" {
			if item.Value.GreaterThanOrEqual(decimal.Zero) {
				return errors.New("Discount value should be less than zero")
			}
		} else {
			if item.Value.LessThanOrEqual(decimal.Zero) {
				return errors.New("Item value should be greater than zero")
			}
		}

		// validate that the total is more than zero
		if i.CalcTotal().LessThanOrEqual(decimal.Zero) {
			return errors.New("The total must be greater than zero")
		}

		// Validate item type
		validType := false
		for _, itemType := range ItemTypes {
			if item.Type == itemType {
				validType = true
				break
			}
		}
		if !validType {
			return errors.New("Invalid item type")
		}
	}

	return nil
}

// AddPublic adds the derived public fields to the Invoice
func (i *Invoice) AddPublic() {
	i.PayTo = i.ID
	i.PartDetected = i.IncomingAmount.IsPositive()
	i.TotalDetected = i.IncomingAmount.GreaterThanOrEqual(i.Total)
	i.TotalConfirmed = (i.PaidHeight > 1)
	i.Unconfirmed = false // XXX meant to indicate if a rollback has occured
	i.Estimate = 0        // XXX meant to estimate time until confirmation
}

func (i *Invoice) ToPublic() PublicInvoice {
	pub := PublicInvoice{
		ID:             i.ID,
		Items:          i.Items,
		Created:        i.Created,
		Total:          i.CalcTotal(),
		PayTo:          i.ID,
		Confirmations:  i.Confirmations,
		PartDetected:   false,
		TotalDetected:  false,
		TotalConfirmed: false,
		Unconfirmed:    false, // XXX meant to indicate if a rollback has occured
		Estimate:       0,     // XXX meant to estimate time until confirmation
	}

	if i.LastPaidAmount.IsPositive() {
		pub.PartDetected = true
	}

	if i.LastPaidAmount.GreaterThanOrEqual(i.Total) {
		pub.TotalDetected = true
	}

	if i.PaidHeight > 1 {
		pub.TotalConfirmed = true
	}

	return pub
}

// This is the address as seen by the public API
type PublicInvoice struct {
	ID             Address    `json:"id"`
	Items          []Item     `json:"items"`
	Created        time.Time  `json:"created"`
	Total          CoinAmount `json:"total"` // Calculated
	PayTo          Address    `json:"pay_to_address"`
	Confirmations  int32      `json:"required_confirmations"`
	PartDetected   bool       `json:"part_payment_detected"`       // Calculated
	TotalDetected  bool       `json:"total_payment_detected"`      // Calculated
	TotalConfirmed bool       `json:"total_payment_confirmed"`     // Calculated
	Unconfirmed    bool       `json:"payment_unconfirmed"`         // Calculated
	Estimate       int        `json:"estimate_seconds_to_confirm"` // Calculated
}
