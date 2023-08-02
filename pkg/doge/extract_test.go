package doge

import (
	"testing"
)

func TestExtract(t *testing.T) {
	extECT(t, "xprv9s21ZrQH143K3QTDL4LXw2F7HEK3wJUD2nW2nRk4stbPy6cq3jPPqjiChkVvvNKmPGJxWUtg6LnF5kejMRNNU3TGtRBeJgk33yuGBxrMPHi", "L52XzL2cMkHxqxBXRyEpnPQZGUs3uKiL3R11XbAdHigRzDozKZeW")
	extECT(t, "xprv9uHRZZhk6KAJC1avXpDAp4MDc3sQKNxDiPvvkX8Br5ngLNv1TxvUxt4cV1rGL5hj6KCesnDYUhd7oWgT11eZG7XnxHrnYeSvkzY7d2bhkJ7", "L5BmPijJjrKbiUfG4zbiFKNqkvuJ8usooJmzuD7Z8dkRoTThYnAT")
}

func extECT(t *testing.T, ext_key string, ec_key string) {
	key, err := ExtractECPrivKeyFromBip32(ext_key)
	if err != nil {
		t.Errorf("ExtractECPrivKeyFromBip32: %v", err)
	}
	if key != ec_key {
		t.Errorf("Base58: extracted EC Key doesn't match: %s vs %s", key, ec_key)
	}
}
