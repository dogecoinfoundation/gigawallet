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
	MakeChildAddress(privkey Privkey, addressIndex uint32, isInternal bool) (Address, error)
	Send(Txn) error
}

type Address string
type Privkey string

type Account struct {
	Address         Address
	Privkey         Privkey
	ForeignID       string
	NextInternalKey uint32
	NextExternalKey uint32
}

func (a Account) GetPublicInfo() AccountPublic {
	return AccountPublic{Address: a.Address, ForeignID: a.ForeignID}
}

type AccountPublic struct {
	Address   Address `json:"id"`
	ForeignID string  `json:"foreign_id"`
}

type Txn struct{}

type Invoice struct {
	// ID is the single-use address that the invoice needs to be paid to.
	ID      Address `json:"id"`
	Account Address `json:"account"` // from Account.Address
	TXID    string  `json:"txid"`
	Vendor  string  `json:"vendor"`
	Items   []Item  `json:"items"`
	// These are used internally to track invoice status.
	KeyIndex      uint32 `json:"-"` // which HD Wallet child-key was generated
	BlockID       string `json:"-"` // transaction seen in this mined block
	Confirmations int32  `json:"-"` // number of confirmed blocks (since block_id)
}

type Item struct {
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Quantity  int             `json:"quantity"`
	ImageLink string          `json:"image_link"`
}
