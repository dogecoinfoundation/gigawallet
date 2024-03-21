package doge

import (
	"fmt"
	"math"
)

var ZeroCoins = NewCoinAmount(0)

type CoinAmount struct {
	val uint64
}

// An amount of Doge in Koinu (int64)
// This is the same encoding used on the blockchain.
// Wrapped in a struct to protect against overflow (actually saturation arithmetic)
// Cannot go below zero. Invalid if above MaxMoney.
func NewCoinAmount(val uint64) CoinAmount {
	return CoinAmount{val}
}

// True if CoinAmount is between 0 and MaxMoney
func (c CoinAmount) IsValid() bool {
	return c.val <= MaxMoney.val
}

func (c CoinAmount) IsPositive() bool {
	return c.val > 0
}

func (c CoinAmount) LessThan(a CoinAmount) bool {
	return c.val < a.val
}

var OneDoge = NewCoinAmount(100_000_000)                            // 1 Doge in Koinu
var MaxMoney = NewCoinAmount(10_000_000_000 * OneDoge.val)          // max transaction 10,000,000,000 Doge
const MaxKoinuDigits = 8                                            // number of Koinu digits after the decimal place (OneDoge)
const MaxInt64Digits = 18                                           // 9[223372036854775807]
const MaxWholeCoinDigits = MaxInt64Digits - MaxKoinuDigits          // number of Coin digits before decimal place (MaxMoney)
var BadMoney = MaxMoney.val + ((math.MaxUint64 - MaxMoney.val) / 2) // above MaxMoney (half-way to MaxUint64 as a guard band)

func (c CoinAmount) ToString() string {
	doges := c.val / OneDoge.val
	koinu := c.val % OneDoge.val
	s := fmt.Sprintf("%d.%08d", doges, koinu)
	return s
}

// func (c CoinAmount) String() string {
// 	return c.ToString()
// }

// Multiply CoinAmount by a number, detecting overflow.
// If overflow happens, this returns a CoinAmount with IsValid false.
func (c CoinAmount) Mul(by uint64) CoinAmount {
	if by != 0 && c.val > math.MaxUint64/by {
		return CoinAmount{BadMoney} // overflow
	}
	return CoinAmount{c.val * by}
}

// Add a CoinAmount, detecting overflow.
// If overflow happens, this returns a CoinAmount with IsValid false.
func (c CoinAmount) Add(add CoinAmount) CoinAmount {
	if add.val <= math.MaxUint64-c.val {
		return CoinAmount{c.val + add.val}
	}
	return CoinAmount{BadMoney} // overflow
}

// Parse a CoinAmount within any struct being json.Unmarshalled.
// Accepts either a decimal string or a json number (both parsed exact)
func (val *CoinAmount) UnmarshalJSON(item []byte) error {
	var str string
	if item[0] == '"' {
		// no unescape, should not be necessary for a number.
		str = string(item[1 : len(item)-2])
	} else {
		str = string(item)
	}
	doge, err := ParseCoinAmount(str)
	*val = doge
	return err
}

// Encode CoinAmount as a string to avoid floating-point rounding.
func (val *CoinAmount) MarshalJSON() ([]byte, error) {
	return []byte("\"" + val.ToString() + "\""), nil
}

// Parse a string-encoded coin amount.
// This format is preferred to avoid floating-point rounding.
func ParseCoinAmount(amt string) (CoinAmount, error) {
	used := 0
	src := amt
	if amt[0] == 0x2d { // minus '-'
		return ZeroCoins, fmt.Errorf("invalid CoinAmount: negative values are not allowed: %v", amt)
	}
	doges, dogelen := parseU64(src)
	used += dogelen
	if dogelen > MaxWholeCoinDigits && !(dogelen == MaxWholeCoinDigits+1 && doges == MaxMoney.val) {
		return ZeroCoins, fmt.Errorf("invalid CoinAmount: greater than MAX_MONEY (10,000,000,000 Doge): %v", amt)
	}
	doges *= OneDoge.val      // checked above <= MaxMoney
	if src[dogelen] == 0x2e { // dot '.'
		koinu, koinlen := parseU64(src[dogelen+1:])
		used += koinlen
		if koinlen > MaxKoinuDigits {
			return ZeroCoins, fmt.Errorf("invalid CoinAmount: more than %v digits (Koinu) after decimal place: %v", MaxKoinuDigits, amt)
		}
		// pad with virtual zeroes until MaxKoinuDigits long,
		// so the parsed digits are interpreted left-aligned, e.g. 0.2 doge from ".2"
		for koinlen < MaxKoinuDigits {
			koinu *= 10
			koinlen += 1
		}
		doges += koinu // checked above < OneDoge
	}
	if used < len(amt) {
		return ZeroCoins, fmt.Errorf("invalid CoinAmount: unexpected character at %v: %v", used, amt)
	}
	val := CoinAmount{doges}
	if val.ToString() != amt {
		return ZeroCoins, fmt.Errorf("invalid CoinAmount: parsed value %v does not match supplied value %v", val.ToString(), amt)
	}
	return val, nil
}

func parseU64(str string) (uint64, int) {
	var n uint64
	rd := 0
	for i, c := range str {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
			rd = i
		} else {
			break
		}
	}
	return n, rd
}
