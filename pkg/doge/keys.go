package doge

import (
	"bytes"
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

var zeroBytes [80]byte // 80 zero-bytes (used in other files)

func clear(slice []byte) {
	if len(slice) > 80 {
		panic("clear: slice too big")
	}
	copy(slice, zeroBytes[0:len(slice)])
}

func GenerateECPrivKey() (ECPrivKey, error) {
	// can return an error if entropy is not available
	pk, err := secp256k1.GeneratePrivateKeyFromRand(rand.Reader)
	if err != nil {
		return nil, err
	}
	ret := pk.Serialize()
	pk.Zero() // clear key for security.
	return ret, nil
}

func ECPubKeyFromECPrivKey(pk ECPrivKey) ECPubKeyCompressed {
	key := secp256k1.PrivKeyFromBytes(pk)
	pub := key.PubKey().SerializeCompressed()
	key.Zero() // clear key for security.
	return pub
}

func ECKeyIsValid(pk ECPrivKey) bool {
	if len(pk) != ECPrivKeyLen {
		return false
	}
	// From secp256k1.PrivKeyFromBytes:
	// "Further, 0 is not a valid private key. It is up to the caller
	// to provide a value in the appropriate range of [1, N-1]."
	if bytes.Equal(pk, zeroBytes[0:ECPrivKeyLen]) {
		return false // zero is not a valid key.
	}
	// If overflow is true, it means the ECPrivKey is >= N (the order
	// of the secp256k1 curve) which is not a strong private key.
	// PrivKeyFromBytes will accept keys >= N, but it will reduce them
	// modulo N, so they become equivalent to a lower key value.
	var modN secp256k1.ModNScalar
	overflow := modN.SetByteSlice(pk)
	modN.Zero() // clear key for security.
	return !overflow
}
