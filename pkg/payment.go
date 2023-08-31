package giga

import "time"

type Payment struct {
	ID               int64      // incrementing payment number, per account
	AccountAddress   Address    // owner account (source of funds)
	PayTo            Address    // a dogecoin address
	Amount           CoinAmount // how much was paid
	Created          time.Time  // when the payment was created
	PaidTxID         string     // TXID of the Transaction that made the payment
	PaidHeight       int64      // Block Height of the Transaction that made the payment
	ConfirmedHeight  int64      // Block Height when payment transaction was confirmed
	OnChainEvent     time.Time  // Time when the on-chain event was sent
	ConfirmedEvent   time.Time  // Time when the confirmed event was sent
	UnconfirmedEvent time.Time  // Time when the unconfirmed event was sent
}
