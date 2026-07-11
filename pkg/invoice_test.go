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

func TestToPublicFlagsUseCurrentAmounts(t *testing.T) {
	// Repro #149: funds present but LastIncomingAmount not advanced (event lag)
	inv := Invoice{
		ID:                 Address("DInvoice"),
		Account:            Address("DAccount"),
		Created:            time.Now(),
		Total:              mustAmt("1"),
		IncomingAmount:     mustAmt("1"),
		PaidAmount:         mustAmt("1"),
		LastIncomingAmount: ZeroCoins,
		LastPaidAmount:     ZeroCoins,
		PaidHeight:         0,
		Confirmations:      1,
		Items: []Item{{
			Type:     "item",
			Name:     "2 Doge Coins",
			SKU:      "70",
			Value:    mustAmt("1"),
			Quantity: 1,
		}},
	}

	pub := inv.ToPublic()
	if !pub.PartDetected {
		t.Fatalf("PartDetected: want true when IncomingAmount > 0")
	}
	if !pub.TotalDetected {
		t.Fatalf("TotalDetected: want true when IncomingAmount >= Total")
	}
	if !pub.TotalConfirmed {
		t.Fatalf("TotalConfirmed: want true when PaidAmount >= Total")
	}
}

func TestToPublicConfirmedByPaidHeight(t *testing.T) {
	inv := Invoice{
		ID:         Address("DInvoice"),
		Total:      mustAmt("5"),
		PaidHeight: 5_000_000,
		PaidAmount: mustAmt("5"),
		Items:      []Item{{Type: "item", Name: "x", Value: mustAmt("5"), Quantity: 1}},
	}
	pub := inv.ToPublic()
	if !pub.TotalConfirmed {
		t.Fatalf("TotalConfirmed: want true when PaidHeight != 0")
	}
}

func TestToPublicUnpaid(t *testing.T) {
	inv := Invoice{
		ID:    Address("DInvoice"),
		Total: mustAmt("2"),
		Items: []Item{{Type: "item", Name: "x", Value: mustAmt("2"), Quantity: 1}},
	}
	pub := inv.ToPublic()
	if pub.PartDetected || pub.TotalDetected || pub.TotalConfirmed {
		t.Fatalf("unpaid invoice should have all flags false: %+v", pub)
	}
}
