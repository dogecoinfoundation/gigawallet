package giga

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func mustAmt(s string) CoinAmount {
	a, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return a
}

func TestAddPublicAndToPublicFlags(t *testing.T) {
	inv := Invoice{
		ID:                 Address("DInvoice"),
		Account:            Address("DAccount"),
		Created:            time.Now(),
		Total:              mustAmt("1"),
		IncomingAmount:     mustAmt("1"),
		PaidAmount:         mustAmt("1"),
		LastIncomingAmount: ZeroCoins,
		PaidHeight:         0,
		Items:              []Item{{Type: "item", Name: "x", Value: mustAmt("1"), Quantity: 1}},
	}
	inv.AddPublic()
	if !inv.TotalConfirmed || !inv.TotalDetected {
		t.Fatalf("AddPublic flags wrong: confirmed=%v detected=%v", inv.TotalConfirmed, inv.TotalDetected)
	}
	pub := inv.ToPublic()
	if !pub.TotalConfirmed || !pub.TotalDetected {
		t.Fatalf("ToPublic flags wrong: confirmed=%v detected=%v", pub.TotalConfirmed, pub.TotalDetected)
	}
}
