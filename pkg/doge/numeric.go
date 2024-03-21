package doge

import "github.com/shopspring/decimal"

const (
	OneDoge = int64(100_000_000) // in Koinu
)

var OneDogeDec = decimal.NewFromInt(OneDoge)

func KoinuToDecimal(koinu int64) decimal.Decimal {
	return decimal.NewFromInt(koinu).Div(OneDogeDec)
}
