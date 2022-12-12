package giga

import (
	"fmt"

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
	MakeTransaction(amount Koinu, UTXOs []UTXO, payTo Address, fee Koinu, change Address) (Txn, error)
	Send(Txn) error
}

type Address string
type Privkey string
type Koinu uint64

const numKoinuDigits int = 8
const oneCoinInKoinu Koinu = 100000000          // 8 koinu zeros
const txnFeePerKB Koinu = oneCoinInKoinu / 100  // 0.01 DOGE
const txnFeePerByte Koinu = txnFeePerKB / 1000  // since Core version 1.14.5
const txnDustLimit Koinu = oneCoinInKoinu / 100 // 0.01 DOGE

func (k Koinu) ToCoinString() string {
	wholeCoins := k / oneCoinInKoinu
	koinuPart := k % oneCoinInKoinu
	return fmt.Sprintf("%d.%*d", wholeCoins, numKoinuDigits, koinuPart)
}

type Account struct {
	Address         Address
	Privkey         Privkey
	ForeignID       string
	NextInternalKey uint32
	NextExternalKey uint32
}

type UTXO struct {
	TxnID string
	VOut  int
	Value Koinu
}

func (a Account) GetPublicInfo() AccountPublic {
	return AccountPublic{Address: a.Address, ForeignID: a.ForeignID}
}

type AccountPublic struct {
	Address   Address `json:"id"`
	ForeignID string  `json:"foreign_id"`
}

type Txn struct {
	TxnHex       string
	InAmount     Koinu
	PayAmount    Koinu
	FeeAmount    Koinu
	ChangeAmount Koinu
}

type Invoice struct {
	// ID is the single-use address that the invoice needs to be paid to.
	ID      Address `json:"id"`      // pay-to Address (Invoice ID)
	Account Address `json:"account"` // an Account.Address (Account ID)
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
