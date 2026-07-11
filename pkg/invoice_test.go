package giga

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestInvoiceExpirePublicFields(t *testing.T) {
	inv := Invoice{
		ID:           Address("Dtest"),
		Items:        []Item{{Type: "item", Name: "x", Value: decimal.NewFromInt(1), Quantity: 1}},
		Created:      time.Now(),
		ExpireHeight: 12345,
		Archived:     true,
	}
	pub := inv.ToPublic()
	if pub.ExpireHeight != 12345 {
		t.Fatalf("ExpireHeight: got %d", pub.ExpireHeight)
	}
	if !pub.Archived {
		t.Fatalf("expected Archived true")
	}
}

func TestInvoiceValidateAndTotal(t *testing.T) {
	inv := Invoice{
		Items: []Item{
			{Type: "item", Name: "widget", Value: decimal.NewFromFloat(1.5), Quantity: 2},
		},
	}
	if err := inv.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !inv.CalcTotal().Equal(decimal.NewFromInt(3)) {
		t.Fatalf("CalcTotal: got %s", inv.CalcTotal())
	}
}
