package doge

import "github.com/btcsuite/golangcrypto/ripemd160"

func RIPEMD160(bytes []byte) []byte {
	hash := ripemd160.New()
	n, err := hash.Write(bytes)
	if err != nil || n != len(bytes) {
		panic("RipEMD160: cannot write bytes")
	}
	var res []byte
	return hash.Sum(res)
}
