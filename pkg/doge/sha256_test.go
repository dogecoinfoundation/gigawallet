package doge

import (
	"strings"
	"testing"
)

func TestSHA256(t *testing.T) {
	// Double SHA-256
	// https://www.dlitz.net/crypto/shad256-test-vectors/SHAd256_Test_Vectors.txt
	// NIST test vectors (see FIPS PUB 180-2 Appendix B)
	shaDT(t, "616263", "4f8b42c22dd3729b519ba6f68d2da7cc5b2d606d05daed5ad5128cc03e6c6358")
	shaDT(t, "6162636462636465636465666465666765666768666768696768696a68696a6b696a6b6c6a6b6c6d6b6c6d6e6c6d6e6f6d6e6f706e6f7071", "0cffe17f68954dac3a84fb1458bd5ec99209449749b2b308b7cb55812f9563af")
	shaDT(t, strings.Repeat("61", 1000000), "80d1189477563e1b5206b2749f1afe4807e5705e8bd77887a60187a712156688") // NB. 0x61 = 'a'
	// NIST SHA-256 Test Vectors Short:
	sha2T(t, "74cb9381d89f5aa73368", "73d6fad1caaa75b43b21733561fd3958bdc555194a037c2addec19dc2d7a52bd")
	sha2T(t, "76ed24a0f40a41221ebfcf", "044cef802901932e46dc46b2545e6c99c0fc323a0ed99b081bda4216857f38ac")
	sha2T(t, "9baf69cba317f422fe26a9a0", "fe56287cd657e4afc50dba7a3a54c2a6324b886becdcd1fae473b769e551a09b")
	sha2T(t, "68511cdb2dbbf3530d7fb61cbc", "af53430466715e99a602fc9f5945719b04dd24267e6a98471f7a7869bd3b4313")
}

func shaDT(t *testing.T, hex string, dbl_sha string) {
	if res := HexEncode(DoubleSha256(hx2b(hex))); res != dbl_sha {
		t.Errorf("DoubleSha256: wrong hash: %s vs %s", res, dbl_sha)
	}
}

func sha2T(t *testing.T, hex string, sha string) {
	if res := HexEncode(Sha256(hx2b(hex))); res != sha {
		t.Errorf("Sha256: wrong hash: %s vs %s", res, sha)
	}
}
