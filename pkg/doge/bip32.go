package doge

import (
	"fmt"
)

// https://en.bitcoin.it/wiki/BIP_0032
type Bip32Key struct {
	keyType      KeyBits  // detected key properties
	version      uint32   // 4 version bytes (ChainParams.bip32_privkey_prefix / bip32_pubkey_prefix)
	depth        byte     // 0x00 for master nodes, 0x01 for level-1 derived keys, ...
	fingerprint  uint32   // the fingerprint of the parent's key (0x00000000 if master key)
	child_number uint32   // child number. ser32(i) for i in xi = xpar/i, with xi the key being serialized. (0x00000000 if master key)
	chain_code   [32]byte // the chain code
	pub_priv_key [33]byte // public key or private key data (serP(K) for public keys, 0x00 || ser256(k) for private keys)
}

const (
	SerializedBip32KeyLength = 4 + 1 + 4 + 4 + 32 + 33
)

func (key *Bip32Key) GetECPrivKey() ([]byte, error) {
	if (key.keyType & keyBip32Priv) == 0 {
		return nil, fmt.Errorf("Bip32Key is not a private key")
	}
	pk := [ECPrivKeyLen]byte{}
	if copy(pk[:], key.pub_priv_key[1:33]) != ECPrivKeyLen {
		panic("GetECPrivKey: wrong length")
	}
	return pk[:], nil
}

func (key *Bip32Key) GetECPubKey() []byte {
	if (key.keyType & keyBip32Priv) != 0 {
		// contains a private key.
		return ECPubKeyFromECPrivKey(key.pub_priv_key[1:33])
	} else {
		// contains a public key.
		pub := [ECPubKeyCompressedLen]byte{}
		if copy(pub[:], key.pub_priv_key[:]) != ECPubKeyCompressedLen {
			panic("GetECPubKey: wrong length")
		}
		return pub[:]
	}
}

func (key *Bip32Key) Clear() {
	key.keyType = 0
	key.version = 0
	key.depth = 0
	key.fingerprint = 0
	key.child_number = 0
	clear(key.chain_code[:])
	clear(key.pub_priv_key[:])
}

// chain is optional, will auto-detect if nil.
func DecodeBip32WIF(extendedKey string, chain *ChainParams) (*Bip32Key, error) {
	data, err := Base58DecodeCheck(extendedKey)
	if err != nil {
		return nil, err
	}
	if len(data) != SerializedBip32KeyLength {
		clear(data) // clear key for security.
		return nil, fmt.Errorf("DecodeBip32WIF: not a bip32 extended key (wrong length)")
	}
	if chain == nil {
		chain = ChainFromWIFPrefix(data, true)
	}
	var key Bip32Key
	key.version = deser32(data[0:])
	if key.version != chain.bip32_privkey_prefix && key.version != chain.bip32_pubkey_prefix {
		clear(data) // clear key for security.
		return nil, fmt.Errorf("DecodeBip32WIF: not a bip32 extended key (wrong prefix)")
	}
	key.depth = data[4]
	key.fingerprint = deser32(data[5:])
	key.child_number = deser32(data[9:])
	if copy(key.chain_code[:], data[13:45]) != 32 {
		panic("DecodeBip32WIF: wrong chain_code length")
	}
	if copy(key.pub_priv_key[:], data[45:78]) != 33 {
		panic("DecodeBip32WIF: wrong key length")
	}
	key.keyType = KeyBitsForChain(chain)
	key_pre := key.pub_priv_key[0]
	if key_pre == 0x00 {
		key.keyType |= keyBip32Priv
	} else if key_pre == 0x02 || key_pre == 0x03 {
		key.keyType |= keyBip32Pub
	} else {
		clear(data) // clear key for security.
		return nil, fmt.Errorf("DecodeBip32WIF: invalid key prefix byte")
	}
	clear(data) // clear key for security.
	return &key, nil
}

func EncodeBip32WIF(key *Bip32Key) (string, error) {
	data := [SerializedBip32KeyLength]byte{}
	ser32(key.version, data[0:4])
	data[4] = key.depth
	ser32(key.fingerprint, data[5:9])
	ser32(key.child_number, data[9:13])
	if copy(data[13:45], key.chain_code[:]) != 32 {
		panic("EncodeBip32WIF: wrong chain_code length")
	}
	if copy(data[45:78], key.pub_priv_key[:]) != 33 {
		panic("EncodeBip32WIF: wrong key length")
	}
	return Base58EncodeCheck(data[:]), nil
}

func ser32(i uint32, to []byte) {
	// serialize a 32-bit unsigned integer, most significant byte first.
	to[0] = byte(i >> 24)
	to[1] = byte(i >> 16)
	to[2] = byte(i >> 8)
	to[3] = byte(i >> 0)
}

func deser32(from []byte) uint32 {
	// deserialize a 32-bit unsigned integer, most significant byte first.
	return (uint32(from[0]) << 24) | (uint32(from[1]) << 16) | (uint32(from[2]) << 8) | (uint32(from[3]))
}
