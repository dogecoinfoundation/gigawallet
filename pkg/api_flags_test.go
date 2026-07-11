package giga

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPaymentFlagsForAML(t *testing.T) {
	a := API{config: TestConfig()}
	a.config.Gigawallet.AMLThresholdUSD = 10000
	a.config.Gigawallet.DogeUSDPrice = 0.1 // 100k DOGE = $10k

	// under threshold
	flags := a.paymentFlagsFor(decimal.NewFromInt(1000), nil) // $100
	if len(flags) != 0 {
		t.Fatalf("expected no flags under threshold, got %v", flags)
	}

	// at threshold: 100000 * 0.1 = 10000
	flags = a.paymentFlagsFor(decimal.NewFromInt(100000), nil)
	if len(flags) != 1 || flags[0] != "AML" {
		t.Fatalf("expected AML flag, got %v", flags)
	}

	// explicit + auto
	flags = a.paymentFlagsFor(decimal.NewFromInt(100000), []string{"REVIEW", "AML"})
	if len(flags) != 2 {
		t.Fatalf("expected 2 unique flags, got %v", flags)
	}
}
