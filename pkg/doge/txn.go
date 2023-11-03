package doge

func TxHashHex(tx []byte) string {
	hash := DoubleSha256(tx)
	reverseInPlace(hash)
	return HexEncode(hash)
}

func reverseInPlace(a []byte) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
}
