package doge

import (
	"bytes"
	"testing"
)

func TestWIF(t *testing.T) {
	pkey := hx2b("0C28FCA386C7A227600B2FE50B7CAE11EC86D3BF1FBE471BE89827E19D72AA1D")
	wif := WIFEncodeECPrivKey(pkey, &MainChain)
	if wif != "QP2GKa5kuU2i2G3xJMH5KL9NErbVYGxMoRiF5trrJJvHzrJ2Ebp7" {
		t.Fatalf("WIFEncodePKey failed: %v", wif)
	}
	key, chain, err := WIFDecodeECPrivKey("QP2GKa5kuU2i2G3xJMH5KL9NErbVYGxMoRiF5trrJJvHzrJ2Ebp7")
	if err != nil {
		t.Fatalf("WIFDecodeECPrivKey: decode failed: %v", err)
	}
	if chain != &MainChain {
		t.Fatalf("WIFDecodeECPrivKey: wrong chain")
	}
	if !bytes.Equal(key, pkey) {
		t.Fatalf("WIFDecodeECPrivKey: decoded bytes differ: %v vs %v", key, pkey)
	}
}
