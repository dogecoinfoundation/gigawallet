package doge

import (
	"crypto/rand"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

const (
	ECPrivKeyLen            = 32 // bytes.
	ECPubKeyCompressedLen   = 33 // bytes: [2/3][32-X] 2=even 3=odd
	ECPubKeyUncompressedLen = 65 // bytes: [4][32-X][32-Y]
)

type ECPrivKey = []byte            // 32 bytes.
type ECPubKeyCompressed = []byte   // 33 bytes.
type ECPubKeyUncompressed = []byte // 65 bytes.

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
