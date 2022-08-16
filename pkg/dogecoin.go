package giga

import (
	"github.com/shopspring/decimal"
)

// L1 represents access to Dogecoin's L1 functionality.
//
// The general idea is that this will eventually be provided by a
// Go binding for the libdogecoin project, however to begin with
// will be implemented via RPC/ZMQ comms to the Dogecoin Core APIs.
type L1 interface {
	MakeAddress() (Address, error)
	Send(Txn) error
}

type Address string

type Account struct {
	PrivKey string
	PubKey  string
}

type Txn struct{}

type Invoice struct {
	// ID is the single-use address that the invoice needs to be paid to.
	ID     Address
	Vendor string `json:"vendor"`
	Items  []Item `json:"items"`
}

type Item struct {
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Quantity  int             `json:"quantity"`
	ImageLink string          `json:"image_link"`
}
