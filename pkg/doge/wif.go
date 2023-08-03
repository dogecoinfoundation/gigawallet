package doge

import "fmt"

func EncodeECPrivKeyWIF(key ECPrivKey, chain *ChainParams) string {
	// https://en.bitcoin.it/wiki/Wallet_import_format
	data := [2 + 32 + 4]byte{}
	data[0] = chain.pkey_prefix
	if copy(data[1:], key) != ECPrivKeyLen {
		panic("EncodeECPrivKeyWIF: wrong key length")
	}
	data[33] = 0x01 // pubkey will be compressed.
	ret := Base58EncodeCheck(data[0:34])
	clear(data[:]) // clear key for security.
	return ret
}

func EncodeECPrivKeyUncompressedWIF(key ECPrivKey, chain *ChainParams) string {
	data := [1 + 32 + 4]byte{}
	data[0] = chain.pkey_prefix
	if copy(data[1:], key) != ECPrivKeyLen {
		panic("EncodeECPrivKeyUncompressedWIF: wrong key length")
	}
	// pubkey will be uncompressed (no 0x01 byte)
	ret := Base58EncodeCheck(data[0:33])
	clear(data[:]) // clear key for security.
	return ret
}

// chain is optional, will auto-detect if nil.
func DecodeECPrivKeyWIF(str string, chain *ChainParams) (ec_priv_key ECPrivKey, out_chain *ChainParams, err error) {
	data, err := Base58DecodeCheck(str)
	if err != nil {
		return nil, nil, err
	}
	if chain == nil {
		chain = ChainFromWIFPrefix(data, true)
	}
	if data[0] != chain.pkey_prefix {
		err = fmt.Errorf("DecodeECPrivKeyWIF: wrong key prefix")
		return nil, nil, err
	}
	var pk [ECPrivKeyLen]byte
	if copy(pk[:], data[1:33]) != ECPrivKeyLen {
		panic("DecodeECPrivKeyWIF: wrong copy length")
	}
	if !ECKeyIsValid(pk[:]) {
		err = fmt.Errorf("DecodeECPrivKeyWIF: invalid EC key (zero or >= N)")
		return nil, nil, err
	}
	clear(data[:]) // clear key for security.
	return pk[:], chain, nil
}
