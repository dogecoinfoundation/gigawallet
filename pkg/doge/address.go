package doge

import (
	"errors"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func Hash160(bytes []byte) []byte {
	return RIPEMD160(Sha256(bytes))
}

func Hash160toAddress(hash []byte, prefix byte) Address {
	ver_hash := [1 + 20 + 4]byte{}
	ver_hash[0] = prefix
	if copy(ver_hash[1:], hash) != 20 {
		panic("PubKeyToP2PKH: wrong RIPEMD-160 length")
	}
	return Address(Base58EncodeCheck(ver_hash[0:21]))
}

func PubKeyToAddress(key []byte, prefix byte) (Address, error) {
	if len(key) == ECPubKeyUncompressedLen && key[0] == 0x04 {
		pubkey, err := secp256k1.ParsePubKey(key)
		if err != nil {
			return "", err
		}
		key = pubkey.SerializeCompressed()
	}
	if len(key) != ECPubKeyCompressedLen || (key[0] != 0x02 && key[0] != 0x03) {
		return "", errors.New("PubKeyToAddress: invalid pubkey")
	}
	payload := Hash160(key[:])
	ver_hash := [1 + 20 + 4]byte{}
	ver_hash[0] = prefix
	if copy(ver_hash[1:], payload) != 20 {
		return "", errors.New("PubKeyToAddress: wrong RIPEMD-160 length")
	}
	return Address(Base58EncodeCheck(ver_hash[0:21])), nil
}

func ScriptToP2SH(redeemScript []byte, chain *ChainParams) Address {
	if len(redeemScript) < 1 {
		panic("ScriptToP2SH: bad script length")
	}
	payload := Hash160(redeemScript)
	ver_hash := [1 + 20 + 4]byte{}
	ver_hash[0] = chain.p2sh_address_prefix
	if copy(ver_hash[1:], payload) != 20 {
		panic("ScriptToP2SH: wrong RIPEMD-160 length")
	}
	return Address(Base58EncodeCheck(ver_hash[0:21]))
}

func ValidateP2PKH(address Address, chain *ChainParams) bool {
	key, err := Base58DecodeCheck(string(address))
	if err != nil {
		return false
	}
	return key[0] == chain.p2pkh_address_prefix
}

func ValidateP2SH(address Address, chain *ChainParams) bool {
	key, err := Base58DecodeCheck(string(address))
	if err != nil {
		return false
	}
	return key[0] == chain.p2sh_address_prefix
}
