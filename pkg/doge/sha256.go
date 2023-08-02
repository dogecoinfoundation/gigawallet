package doge

import "crypto/sha256"

type Hash256 = []byte

func Sha256(bytes []byte) Hash256 {
	result := sha256.Sum256(bytes)
	return result[:]
}

func DoubleSha256(bytes []byte) Hash256 {
	hash := sha256.Sum256(bytes)
	result := sha256.Sum256(hash[:])
	return result[:]
}
