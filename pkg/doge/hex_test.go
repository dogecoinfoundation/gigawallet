package doge

import (
	"bytes"
	"testing"
)

func TestHex(t *testing.T) {
	data := []byte("\x00\x01Hello\xffWorld!\x0d\x0a")
	hex := HexEncode(data)
	if hex != "000148656c6c6fff576f726c64210d0a" {
		t.Errorf("HexEncode: wrong hex: " + hex)
	}
	out, err := HexDecode(hex)
	if err != nil {
		t.Errorf("HexDecode: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Errorf("HexDecode: decoded bytes don't match: %v vs %v", out, data)
	}
}

// Test Heplers

func hx2b(str string) (bytes []byte) {
	bytes, err := HexDecode(str)
	if err != nil {
		panic("WIF: bad fixture: " + str)
	}
	return
}
