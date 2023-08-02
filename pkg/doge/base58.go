package doge

import (
	"fmt"

	"github.com/mr-tron/base58"
)

func Base58Encode(bytes []byte) string {
	// https://digitalbazaar.github.io/base58-spec/
	return base58.FastBase58Encoding(bytes)
}

// CAUTION: appends the Checksum to `bytes` if it has sufficient capacity (4 bytes)
func Base58EncodeCheck(bytes []byte) string {
	// https://en.bitcoin.it/Base58Check_encoding
	sum := DoubleSha256(bytes)
	bytes = append(bytes, sum[0], sum[1], sum[2], sum[3])
	return base58.FastBase58Encoding(bytes)
}

func Base58Decode(str string) ([]byte, error) {
	bytes, err := base58.FastBase58Decoding(str)
	return bytes, err
}

func Base58DecodeCheck(str string) ([]byte, error) {
	data, err := Base58Decode(str)
	if err != nil {
		return nil, err
	}
	err = Base58VerifyChecksum(data, str)
	if err != nil {
		return nil, err
	}
	return data[0 : len(data)-4], nil
}

func Base58VerifyChecksum(bytes []byte, str string) error {
	// https://en.bitcoin.it/Base58Check_encoding
	if len(bytes) < 5 {
		return fmt.Errorf("Base58Check: too short")
	}
	split := len(bytes) - 4
	payload := bytes[0:split]
	check := bytes[split:]
	sum := DoubleSha256(payload)
	if check[0] != sum[0] || check[1] != sum[1] || check[2] != sum[2] || check[3] != sum[3] {
		return fmt.Errorf("Base58Check: wrong checksum")
	}
	// check leading zeros.
	i := 0
	for i < split && bytes[i] == 0 && str[i] == '1' {
		i += 1
	}
	if bytes[i] == 0 || str[i] == '1' {
		return fmt.Errorf("Base58Check: wrong padding")
	}
	return nil
}
