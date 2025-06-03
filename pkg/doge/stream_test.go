package doge

import (
	"bytes"
	"testing"
)

func TestStreamBytes(t *testing.T) {
	stream := NewStream(hx2b("01020304"))
	if !bytes.Equal(stream.Bytes(4), hx2b("01020304")) {
		t.Errorf("Bytes: wrong value: %x", stream.Bytes(2))
	}
	if !stream.Complete() {
		t.Errorf("Bytes: stream not complete")
	}
}

func TestStreamUint16le(t *testing.T) {
	stream := NewStream(hx2b("0102"))
	if stream.Uint16le() != 0x0201 {
		t.Errorf("Uint16le: wrong value: %x", stream.Uint16le())
	}
	if !stream.Complete() {
		t.Errorf("Uint16le: stream not complete")
	}
}

func TestStreamUint32le(t *testing.T) {
	stream := NewStream(hx2b("01020304"))
	if stream.Uint32le() != 0x04030201 {
		t.Errorf("Uint32le: wrong value: %x", stream.Uint32le())
	}
	if !stream.Complete() {
		t.Errorf("Uint32le: stream not complete")
	}
}

func TestStreamUint64le(t *testing.T) {
	stream := NewStream(hx2b("0102030405060708"))
	if stream.Uint64le() != 0x0807060504030201 {
		t.Errorf("Uint64le: wrong value: %x", stream.Uint64le())
	}
	if !stream.Complete() {
		t.Errorf("Uint64le: stream not complete")
	}
}

func TestStreamVarUint(t *testing.T) {
	// Test VarUint with 1 byte
	stream := NewStream(hx2b("01"))
	if stream.VarUint() != 0x01 {
		t.Errorf("VarUint: wrong value: %x", stream.VarUint())
	}
	if !stream.Complete() {
		t.Errorf("VarUint: stream not complete")
	}
}

func TestStreamVarUint2(t *testing.T) {
	// Test VarUint with 2 bytes
	stream := NewStream(hx2b("FD0102"))
	if stream.VarUint() != 0x0201 {
		t.Errorf("VarUint: wrong value: %x", stream.VarUint())
	}
	if !stream.Complete() {
		t.Errorf("VarUint: stream not complete")
	}
}

func TestStreamVarUint3(t *testing.T) {
	// Test VarUint with 4 bytes
	stream := NewStream(hx2b("FE01020304"))
	if stream.VarUint() != 0x04030201 {
		t.Errorf("VarUint: wrong value: %x", stream.VarUint())
	}
	if !stream.Complete() {
		t.Errorf("VarUint: stream not complete")
	}
}

func TestStreamVarUint4(t *testing.T) {
	// Test VarUint with 8 bytes
	stream := NewStream(hx2b("FF0102030405060708"))
	if stream.VarUint() != 0x0807060504030201 {
		t.Errorf("VarUint: wrong value: %x", stream.VarUint())
	}
	if !stream.Complete() {
		t.Errorf("VarUint: stream not complete")
	}
}

func TestOverrunBytes(t *testing.T) {
	// Test overrun of bytes
	stream := NewStream(hx2b("01020304"))
	if stream.Bytes(5) != nil {
		t.Errorf("Bytes: should return nil for overrun")
	}
	if stream.Valid() {
		t.Errorf("Bytes: stream should not be valid for overrun")
	}
	if stream.Complete() {
		t.Errorf("Bytes: stream should not be complete for overrun")
	}
}

func TestOverrunUint16le(t *testing.T) {
	// Test overrun of Uint16le
	stream := NewStream(hx2b("01"))
	if stream.Uint16le() != 0 {
		t.Errorf("Uint16le: stream should return 0 for overrun")
	}
	if stream.Valid() {
		t.Errorf("Uint16le: stream should not be valid for overrun")
	}
	if stream.Complete() {
		t.Errorf("Uint16le: stream should not be complete for overrun")
	}
}

func TestOverrunUint32le(t *testing.T) {
	// Test overrun of Uint32le
	stream := NewStream(hx2b("010203"))
	if stream.Uint32le() != 0 {
		t.Errorf("Uint32le: stream should return 0 for overrun")
	}
	if stream.Valid() {
		t.Errorf("Uint32le: stream should not be valid for overrun")
	}
	if stream.Complete() {
		t.Errorf("Uint32le: stream should not be complete for overrun")
	}
}

func TestOverrunUint64le(t *testing.T) {
	// Test overrun of Uint64le
	stream := NewStream(hx2b("01020304050607"))
	if stream.Uint64le() != 0 {
		t.Errorf("Uint64le: stream should return 0 for overrun")
	}
	if stream.Valid() {
		t.Errorf("Uint64le: stream should not be valid for overrun")
	}
	if stream.Complete() {
		t.Errorf("Uint64le: stream should not be complete for overrun")
	}
}

func TestOverrunVarUint(t *testing.T) {
	// Test overrun of VarUint
	stream := NewStream([]byte{})
	if stream.VarUint() != 0 {
		t.Errorf("VarUint: should return 0 for overrun")
	}
	if stream.Valid() {
		t.Errorf("VarUint: stream should not be valid for overrun")
	}
	if stream.Complete() {
		t.Errorf("VarUint: stream should not be complete for overrun")
	}
}
