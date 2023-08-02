package doge

import (
	"crypto/rand"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

type KeyBits = int // keyECPriv,keyECPub,keyBip32Priv,keyBip32Pub,dogeMainNet,dogeTestNet
const (
	keyNone      = 0
	keyECPriv    = 1
	keyECPub     = 2
	keyBip32Priv = 4
	keyBip32Pub  = 8
	mainNetDoge  = 16
	testNetDoge  = 32
	mainNetBtc   = 64
)

const (
	ECPrivKeyLen            = 32 // bytes.
	ECPubKeyCompressedLen   = 33 // bytes: [x02/x03][32-X] 2=even 3=odd
	ECPubKeyUncompressedLen = 65 // bytes: [x04][32-X][32-Y]
)

type ECPrivKey = []byte            // 32 bytes.
type ECPubKeyCompressed = []byte   // 33 bytes with 0x02 or 0x03 prefix.
type ECPubKeyUncompressed = []byte // 65 bytes with 0x04 prefix.

func GenerateECPrivKey() (ECPrivKey, error) {
	// can return an error if entropy is not available
	pk, err := secp256k1.GeneratePrivateKeyFromRand(rand.Reader)
	if err != nil {
		return nil, err
	}
	return pk.Serialize(), err
}

func ECPubKeyFromECPrivKey(pk ECPrivKey) ECPubKeyCompressed {
	return secp256k1.PrivKeyFromBytes(pk).PubKey().SerializeCompressed()
}
