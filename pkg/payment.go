package giga

import (
	"time"
)

type Payment struct {
	ID               int64      // incrementing payment number, per account
	AccountAddress   Address    // owner account (source of funds)
	PayTo            []PayTo    // dogecoin addresses and amounts
	Total            CoinAmount // total paid to others (excluding fees and change)
	Fee              CoinAmount // fee paid by the transaction
	Created          time.Time  // when the payment was created
	PaidTxID         string     // TXID of the Transaction that made the payment
	PaidHeight       int64      // Block Height of the Transaction that made the payment
	ConfirmedHeight  int64      // Block Height when payment transaction was confirmed
	OnChainEvent     time.Time  // Time when the on-chain event was sent
	ConfirmedEvent   time.Time  // Time when the confirmed event was sent
	UnconfirmedEvent time.Time  // Time when the unconfirmed event was sent
}

// Pay an amount to an address
// optional DeductFeePercent deducts a percentage of required fees from each PayTo (should sum to 100)
type PayTo struct {
	Amount           CoinAmount `json:"amount"`
	PayTo            Address    `json:"to"`
	DeductFeePercent CoinAmount `json:"deduct_fee_percent"`
}
