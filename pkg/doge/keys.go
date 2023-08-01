package doge

const (
	PrivKeyLen            = 32 // bytes.
	PubKeyCompressedLen   = 33 // bytes: [2/3][32-X] 2=even 3=odd
	PubKeyUncompressedLen = 65 // bytes: [4][32-X][32-Y]
)

type PrivKey = []byte            // 32 bytes.
type PubKeyCompressed = []byte   // 33 bytes.
type PubKeyUncompressed = []byte // 65 bytes.
