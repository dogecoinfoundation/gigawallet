package doge

import (
	"bytes"
	"testing"
)

func TestWIF(t *testing.T) {
	// https://en.bitcoin.it/wiki/Wallet_import_format
	// uncompressed encoding.
	wifUT(t, "0C28FCA386C7A227600B2FE50B7CAE11EC86D3BF1FBE471BE89827E19D72AA1D", "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ")
	// https://learnmeabitcoin.com/technical/wif
	wifUT(t, "b9cf78edb465467793a7abf0e221d0deb333b4ff0ad255f148e42dc4798438a5", "5KE7rpamqCzN9j3c8xpbsCAYosqHRjWAp7ZsQFP8r76knsc5sCQ")
	wifUT(t, "8f2d3a553d5eef1be0713685735dd34d82d80aa328a492535311f270fc409770", "5JuLqXKR21exhvGj2t5m4ArxKvnzyoJJaRucWv3nrQLuy5iM6VV")
	wifUT(t, "d6e8a8d82e1357e6484aab512e91bae58ba5a6f4589d42fdb87c07ee0e73416c", "5KSw91z8PQegim76fND4G86DJNLuxW1iQAxYdByB9b4ZMUmkhWg")
	// compressed encoding.
	// https://learnmeabitcoin.com/technical/wif
	wifCT(t, "ef235aacf90d9f4aadd8c92e4b2562e1d9eb97f0df9ba3b508258739cb013db2", "L5EZftvrYaSudiozVRzTqLcHLNDoVn7H5HSfM9BAN6tMJX8oTWz6")
	wifCT(t, "f238f9b098836c359533e33584feeb058ca5d27bbf6b64ccaf89434f8e79cdbd", "L5LZRi3zs922fvUX9Ns5zDab7jPbNCvBi6uoJjJZ3sjcuhK87CPf")
	wifCT(t, "9f39d6e52683b815ce142b48988867f5e6d0170ab075b32d2e76a8f53e470e40", "L2ZE28yS1hsWv5XeYxdnEZj65ai2UcnLcEHeCySj5YAmhJTbqQZW")
	wifCT(t, "2308075232e174d22c2b9b8a4c3489ef846983b0cb39892e92e09480a136b7ab", "KxPomusAkedagKPkEFtw6iYXQuBv7SQq1cs3bh2mKEJGu8zba7nF")
}

func wifCT(t *testing.T, pkey string, wif string) {
	pkey_c := hx2b(pkey)
	wif_c := EncodeECPrivKeyWIF(pkey_c, &BitcoinMainChain)
	if wif_c != wif {
		t.Fatalf("EncodeECPrivKeyWIF: wrong result: %s vs %s", wif_c, wif)
	}
	key_c, chain_c, err := DecodeECPrivKeyWIF(wif_c, nil)
	if err != nil {
		t.Fatalf("DecodeECPrivKeyWIF: decode failed: %v", err)
	}
	if chain_c != &BitcoinMainChain {
		t.Fatalf("DecodeECPrivKeyWIF: wrong chain")
	}
	if !bytes.Equal(key_c, pkey_c) {
		t.Fatalf("DecodeECPrivKeyWIF: decoded bytes differ: %v vs %v", key_c, pkey)
	}
}

func wifUT(t *testing.T, pkey string, wif string) {
	pkey_u := hx2b(pkey)
	wif_u := EncodeECPrivKeyUncompressedWIF(pkey_u, &BitcoinMainChain)
	if wif_u != wif {
		t.Fatalf("EncodeECPrivKeyUncompressedWIF: wrong result: %s vs %s", wif_u, wif)
	}
	key_u, chain_u, err := DecodeECPrivKeyWIF(wif_u, nil)
	if err != nil {
		t.Fatalf("DecodeECPrivKeyWIF: decode failed: %v", err)
	}
	if chain_u != &BitcoinMainChain {
		t.Fatalf("DecodeECPrivKeyWIF: wrong chain")
	}
	if !bytes.Equal(key_u, pkey_u) {
		t.Fatalf("DecodeECPrivKeyWIF: decoded bytes differ: %v vs %v", key_u, pkey)
	}
}
