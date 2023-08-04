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

func IsValidHex(hex string) bool {
	// eh, this will do.
	_, err := HexDecode(hex)
	return err == nil
}
