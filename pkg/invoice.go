package giga

import (
	"errors"
	"fmt"
	"time"

	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
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
	KeyIndex           uint32     `json:"-"` // which HD Wallet child-key was generated
	BlockID            string     `json:"-"` // transaction seen in this mined block
	PaidHeight         int64      `json:"-"` // block-height when the invoice was marked as paid
	PaidEvent          time.Time  `json:"-"` // timestamp when INV_PAID event was sent
	IncomingAmount     CoinAmount `json:"-"` // total of all incoming UTXOs
	PaidAmount         CoinAmount `json:"-"` // total of all confirmed UTXOs
	LastIncomingAmount CoinAmount `json:"-"` // last incoming total used to send an event
	LastPaidAmount     CoinAmount `json:"-"` // last confirmed total used to send an event
}

// CalcTotal sums up the Items listed on the Invoice.
func (i *Invoice) CalcTotal() CoinAmount {
	total := doge.ZeroCoins
	for _, item := range i.Items {
		total = total.Add(item.Value.Mul(uint64(item.Quantity))) // Quantity > 0
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
			if item.Value >= 0 {
				return errors.New("Discount value should be less than zero")
			}
		} else {
			if item.Value <= 0 {
				return errors.New("Item value should be greater than zero")
			}
		}

		// validate that the total is more than zero
		if !i.CalcTotal().IsPositive() {
			return errors.New("The total must be greater than zero")
		}

		// validate that the total is less than max money
		if i.CalcTotal().IsValid() {
			return fmt.Errorf("The total must be no more than MaxMoney (%v)", doge.MaxMoney.ToString())
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
		Unconfirmed:    false,
		Estimate:       0,
	}

	if i.LastIncomingAmount > 0 {
		pub.PartDetected = true
	}

	if i.LastIncomingAmount >= i.Total {
		pub.TotalDetected = true
	}

	//if i.LastPaidAmount.GreaterThanOrEqual(i.Total) {
	if i.PaidHeight > 1 {
		pub.TotalConfirmed = true
	}

	// TODO: we still need a way to handle Unconfirmed?
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
