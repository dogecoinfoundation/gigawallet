package giga

import "time"

type Payment struct {
	ID             int // auto incrementing key, consider TXID
	AccountAddress Address
	PayTo          Address    // a dogecoin address
	Amount         CoinAmount // how much was paid
	Verified       string     // TXID / Block Height? TODO Raffe
	Created        time.Time
}
