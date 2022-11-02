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
	MakeAddress() (Address, Privkey, error)
	MakeChildAddress(privkey Privkey) (Address, error)
	Send(Txn) error
}

type Address string
type Privkey string

type Account struct {
	Address   Address
	Privkey   Privkey
	ForeignID string
}

func (a Account) GetPublicInfo() AccountPublic {
	return AccountPublic{Address: a.Address, ForeignID: a.ForeignID}
}

type AccountPublic struct {
	Address   Address
	ForeignID string
}

type Txn struct{}

type Invoice struct {
	// ID is the single-use address that the invoice needs to be paid to.
	ID     Address `json:"id"`
	TXID   string  `json:"txid"`
	Vendor string  `json:"vendor"`
	Items  []Item  `json:"items"`
}

type Item struct {
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Quantity  int             `json:"quantity"`
	ImageLink string          `json:"image_link"`
}
