package doge

import (
	"strings"
	"testing"
)

func TestRIPEMD160(t *testing.T) {
	// https://homes.esat.kuleuven.be/~bosselae/ripemd160.html
	// https://gist.github.com/Sajjon/95f0c72ca72f0b6985d550b80eff536d
	ripemdT(t, "", "9c1185a5c5e9fc54612808977ee8f548b2258d31")
	ripemdT(t, "a", "0bdc9d2d256b3ee9daae347be6f4dc835a467ffe")
	ripemdT(t, "abc", "8eb208f7e05d987a9b044a8e98c6b087f15a0bfc")
	ripemdT(t, "message digest", "5d0689ef49d2fae572b881b123a85ffa21595f36")
	ripemdT(t, "abcdefghijklmnopqrstuvwxyz", "f71c27109c692c1b56bbdceb5b9d2865b3708dbc")
	ripemdT(t, "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq", "12a053384a9c0c88e405a06c27dcf49ada62eb2b")
	ripemdT(t, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "b0e20b6e3116640286ed3a87a5713079b21f5189")
	ripemdT(t, strings.Repeat("1234567890", 8), "9b752e45573d4b39f4dbd3323cab82bf63326bfb")
	ripemdT(t, strings.Repeat("a", 1000000), "52783243c1697bdbe16d37f97f68f08325dc1528")
}

func ripemdT(t *testing.T, msg string, hash string) {
	res := HexEncode(RIPEMD160([]byte(msg)))
	if res != hash {
		t.Errorf("RIPEMD160: %s hashed to %s instead of %s", msg, res, hash)
	}
}
