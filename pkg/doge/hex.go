package doge

import (
	"encoding/hex"
)

func HexEncode(bytes []byte) string {
	return hex.EncodeToString(bytes)
}

func HexDecode(str string) ([]byte, error) {
	return hex.DecodeString(str)
}
