package giga

import "time"

type Payment struct {
	ID             int64      // incrementing payment number, per account
	AccountAddress Address    // owner account (source of funds)
	PayTo          Address    // a dogecoin address
	Amount         CoinAmount // how much was paid
	Created        time.Time  // when the payment was created
	PaidTxID       string     // TXID of the Transaction that made the payment
	PaidHeight     int64      // Block Height of the Transaction that made the payment
	NotifyHeight   int64      // Block Height when Paid Notification Event was generated
}
