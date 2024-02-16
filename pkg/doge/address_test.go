package doge

import (
	"testing"
)

func TestAddress(t *testing.T) {
	// https://en.bitcoin.it/wiki/Technical_background_of_version_1_Bitcoin_addresses
	// priv := hx2b("18e14a7b6a307f426a94f8114701e7c8e774e7f9a47e2c2035db29a206321725")
	pub := hx2b("0250863ad64a87ae8a2fe83c1af1a8403cb53f53e486d8511dad8a04887e5b2352")
	p2pkh, err := PubKeyToP2PKH(pub, &BitcoinMainChain)
	if err != nil {
		t.Fatalf("PubKeyToP2PKH: %v", err)
	}
	if p2pkh != "1PMycacnJaSqwwJqjawXBErnLsZ7RkXUAs" {
		t.Fatalf("PubKeyToP2PKH is wrong: %s", p2pkh)
	}
}
