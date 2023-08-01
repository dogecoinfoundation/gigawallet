package doge

import "fmt"

func WIFEncodeECPrivKey(key PrivKey, chain *ChainParams) string {
	// https://en.bitcoin.it/wiki/Wallet_import_format
	data := [2 + 32 + 4]byte{}
	data[0] = chain.pkey_prefix
	if copy(data[1:], key) != PrivKeyLen {
		panic("WIFEncodePKey: wrong key length")
	}
	data[33] = 0x01 // pubkey will be compressed.
	return Base58EncodeCheck(data[0:34])
}

func WIFEncodeECPrivKeyUncompressed(key PrivKey, chain *ChainParams) string {
	data := [1 + 32 + 4]byte{}
	data[0] = chain.pkey_prefix
	if copy(data[1:], key) != PrivKeyLen {
		panic("WIFEncodePKey: wrong key length")
	}
	// pubkey will be uncompressed (no 0x01 byte)
	return Base58EncodeCheck(data[0:33])
}

func WIFDecodeECPrivKey(str string) (PrivKey, *ChainParams, error) {
	data, err := Base58DecodeCheck(str)
	if err != nil {
		return nil, nil, err
	}
	chain := ChainFromWIFPrefix(data)
	if data[0] != chain.pkey_prefix {
		err = fmt.Errorf("WIFDecodePKey: wrong key prefix")
		return nil, nil, err
	}
	var pk [PrivKeyLen]byte
	if copy(pk[:], data[1:33]) != PrivKeyLen {
		panic("WIFDecodePKey: wrong copy length")
	}
	return pk[:], chain, nil
}
