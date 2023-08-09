package giga

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Invoice is a request for payment created by Gigawallet.
type Invoice struct {
	// ID is the single-use address that the invoice needs to be paid to.
	ID            Address `json:"id"`      // pay-to Address (Invoice ID)
	Account       Address `json:"account"` // an Account.Address (Account ID)
	TXID          string  `json:"txid"`
	Vendor        string  `json:"vendor"`
	Items         []Item  `json:"items"`
	Confirmations int32   `json:"confirmations"` // number of confirmed blocks (since block_id)
	// These are used internally to track invoice status.
	KeyIndex uint32    `json:"-"` // which HD Wallet child-key was generated
	BlockID  string    `json:"-"` // transaction seen in this mined block
	Created  time.Time `json:"created"`
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
	Type        string          `json:"type"` //ItemTypes
	Name        string          `json:"name"`
	SKU         string          `json:"sku"`
	Description string          `json:"description"`
	Value       decimal.Decimal `json:"value"`
	Quantity    int             `json:"quantity"`
	ImageLink   string          `json:"image_link"`
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
