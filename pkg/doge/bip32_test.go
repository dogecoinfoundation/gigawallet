package doge

import (
	"bytes"
	"testing"
)

func TestBip32(t *testing.T) {
	// https://en.bitcoin.it/wiki/BIP_0032 (Test Vectors)
	bip32T(t,
		"xpub661MyMwAqRbcFtXgS5sYJABqqG9YLmC4Q1Rdap9gSE8NqtwybGhePY2gZ29ESFjqJoCu1Rupje8YtGqsefD265TMg7usUDFdp6W1EGMcet8",
		"xprv9s21ZrQH143K3QTDL4LXw2F7HEK3wJUD2nW2nRk4stbPy6cq3jPPqjiChkVvvNKmPGJxWUtg6LnF5kejMRNNU3TGtRBeJgk33yuGBxrMPHi",
		0, 0)
	bip32T(t,
		"xpub68Gmy5EdvgibQVfPdqkBBCHxA5htiqg55crXYuXoQRKfDBFA1WEjWgP6LHhwBZeNK1VTsfTFUHCdrfp1bgwQ9xv5ski8PX9rL2dZXvgGDnw",
		"xprv9uHRZZhk6KAJC1avXpDAp4MDc3sQKNxDiPvvkX8Br5ngLNv1TxvUxt4cV1rGL5hj6KCesnDYUhd7oWgT11eZG7XnxHrnYeSvkzY7d2bhkJ7",
		1, 0x80000000)
	bip32T(t,
		"xpub6ASuArnXKPbfEwhqN6e3mwBcDTgzisQN1wXN9BJcM47sSikHjJf3UFHKkNAWbWMiGj7Wf5uMash7SyYq527Hqck2AxYysAA7xmALppuCkwQ",
		"xprv9wTYmMFdV23N2TdNG573QoEsfRrWKQgWeibmLntzniatZvR9BmLnvSxqu53Kw1UmYPxLgboyZQaXwTCg8MSY3H2EU4pWcQDnRnrVA1xe8fs",
		2, 1)
	bip32T(t,
		"xpub6D4BDPcP2GT577Vvch3R8wDkScZWzQzMMUm3PWbmWvVJrZwQY4VUNgqFJPMM3No2dFDFGTsxxpG5uJh7n7epu4trkrX7x7DogT5Uv6fcLW5",
		"xprv9z4pot5VBttmtdRTWfWQmoH1taj2axGVzFqSb8C9xaxKymcFzXBDptWmT7FwuEzG3ryjH4ktypQSAewRiNMjANTtpgP4mLTj34bhnZX7UiM",
		3, 0x80000002)
	bip32T(t,
		"xpub6FHa3pjLCk84BayeJxFW2SP4XRrFd1JYnxeLeU8EqN3vDfZmbqBqaGJAyiLjTAwm6ZLRQUMv1ZACTj37sR62cfN7fe5JnJ7dh8zL4fiyLHV",
		"xprvA2JDeKCSNNZky6uBCviVfJSKyQ1mDYahRjijr5idH2WwLsEd4Hsb2Tyh8RfQMuPh7f7RtyzTtdrbdqqsunu5Mm3wDvUAKRHSC34sJ7in334",
		4, 2)
	bip32T(t,
		"xpub6H1LXWLaKsWFhvm6RVpEL9P4KfRZSW7abD2ttkWP3SSQvnyA8FSVqNTEcYFgJS2UaFcxupHiYkro49S8yGasTvXEYBVPamhGW6cFJodrTHy",
		"xprvA41z7zogVVwxVSgdKUHDy1SKmdb533PjDz7J6N6mV6uS3ze1ai8FHa8kmHScGpWmj4WggLyQjgPie1rFSruoUihUZREPSL39UNdE3BBDu76",
		5, 1000000000)
}

// ec_key has 0x00 prefix for private, 0x02/0x03 for public.
func bip32T(t *testing.T, xpub string, xpriv string, depth byte, child_number uint32) {
	// decode.
	pub, err := DecodeBip32WIF(xpub, nil)
	if err != nil {
		t.Errorf("DecodeBip32WIF: xpub: %v", err)
	}
	priv, err := DecodeBip32WIF(xpriv, nil)
	if err != nil {
		t.Errorf("DecodeBip32WIF: xpriv: %v", err)
	}
	// check depth and child_number.
	if pub.depth != depth {
		t.Errorf("Bip32WIF: xpub has wrong key depth: %d vs %d for %s", pub.depth, depth, xpub)
	}
	if priv.depth != depth {
		t.Errorf("Bip32WIF: xpriv has wrong key depth: %d vs %d for %s", priv.depth, depth, xpriv)
	}
	if pub.child_number != child_number {
		t.Errorf("Bip32WIF: xpub has wrong child_number: %d vs %d for %s", pub.child_number, child_number, xpub)
	}
	if priv.child_number != child_number {
		t.Errorf("Bip32WIF: xpriv has wrong child_number: %d vs %d for %s", priv.child_number, child_number, xpriv)
	}
	// check EC keypair.
	pubFromPriv := priv.GetECPubKey()
	if !bytes.Equal(pubFromPriv, pub.pub_priv_key[:]) {
		t.Errorf("Bip32WIF: xpriv EC Key does not generate xpub EC Key: %s vs %s for %s", HexEncode(pubFromPriv), HexEncode(pub.pub_priv_key[:]), xpriv)
	}
	// check re-encode.
	xpub_2, err := EncodeBip32WIF(pub)
	if err != nil {
		t.Errorf("EncodeBip32WIF: %v for %s", err, xpub)
	}
	if xpub_2 != xpub {
		t.Errorf("Bip32WIF: xpub did not round-trip: %s vs %s", xpub_2, xpub)
	}
	xpriv_2, err := EncodeBip32WIF(priv)
	if err != nil {
		t.Errorf("EncodeBip32WIF: %v for %s", err, xpriv)
	}
	if xpriv_2 != xpriv {
		t.Errorf("Bip32WIF: xpriv did not round-trip: %s vs %s", xpriv_2, xpriv)
	}
}
