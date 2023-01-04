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
	MakeTransaction(amount CoinAmount, UTXOs []UTXO, payTo Address, fee CoinAmount, change Address, private_key Privkey) (NewTxn, error)
	DecodeTransaction(txnHex string) (DecodedTxn, error)
	Send(NewTxn) error
}

type Address string
type Privkey string
type CoinAmount = decimal.Decimal

var ZeroCoins = decimal.NewFromInt(0)                         // 0 DOGE
var OneCoin = decimal.NewFromInt(1)                           // 1.0 DOGE
var TxnFeePerKB = OneCoin.Div(decimal.NewFromInt(100))        // 0.01 DOGE
var TxnFeePerByte = TxnFeePerKB.Div(decimal.NewFromInt(1000)) // since Core version 1.14.5
var TxnDustLimit = OneCoin.Div(decimal.NewFromInt(100))       // 0.01 DOGE

// Account is a single user account (Wallet) managed by Gigawallet.
type Account struct {
	Address         Address
	Privkey         Privkey
	ForeignID       string
	NextInternalKey uint32
	NextExternalKey uint32
}

// NextChangeAddress generates the next unused "internal address"
// in the Account's HD-Wallet keyspace. NOTE: since callers don't run
// inside a transaction, concurrent requests can end up paying to the
// same change address (we accept this risk)
func (a *Account) NextChangeAddress(lib L1) (Address, error) {
	keyIndex := a.NextInternalKey
	address, err := lib.MakeChildAddress(a.Privkey, keyIndex, true)
	if err != nil {
		return "", err
	}
	return address, nil
}

// UnreservedUTXOs creates an iterator over UTXOs in this Account that
// have not already been earmarked for an outgoing payment (i.e. reserved.)
// UTXOs are fetched incrementally from the Store, because there can be
// a lot of them. This should iterate in desired spending order.
// NOTE: this does not reserve the UTXOs returned; the caller must to that
// by calling Store.CreateTransaction with the selcted UTXOs - and that may
// fail if the UTXOs have been reserved by a concurrent request. In that case,
// the caller should start over with a new UnreservedUTXOs() call.
func (a *Account) UnreservedUTXOs(s Store) (iter UTXOIterator, err error) {
	// TODO: change this to fetch UTXOs from the Store in batches
	// using a paginated query API.
	allUTXOs, err := s.GetAllUnreservedUTXOs(a.Address)
	if err != nil {
		return &AccountUnspentUTXOs{}, err
	}
	return &AccountUnspentUTXOs{utxos: allUTXOs, next: 0}, nil
}

type AccountUnspentUTXOs struct {
	utxos []UTXO
	next  int
}

func (it *AccountUnspentUTXOs) hasNext() bool {
	return it.next < len(it.utxos)
}
func (it *AccountUnspentUTXOs) getNext() UTXO {
	utxo := it.utxos[it.next]
	it.next++
	return utxo
}

// GetPublicInfo gets those parts of the Account that are safe
// to expose to the outside world (i.e. NOT private keys)
func (a Account) GetPublicInfo() AccountPublic {
	return AccountPublic{Address: a.Address, ForeignID: a.ForeignID}
}

type AccountPublic struct {
	Address   Address `json:"id"`
	ForeignID string  `json:"foreign_id"`
}

// UTXO is an Unspent Transaction Output, i.e. a prior payment into our Account.
type UTXO struct {
	Account       Address    // receiving account ID (by matching ScriptAddress against account's HD child keys)
	TxnID         string     // is an output from this Txn ID
	VOut          int        // is an output at this index in Txn
	Status        string     // 'p' = receive pending; 'c' = receive confirmed; 's' = spent pending; 'x' = spent confirmed
	Value         CoinAmount // value of the txn output in dogecoin
	ScriptType    string     // 'p2pkh', 'multisig', etc (by pattern-matching the txn output script code)
	ScriptAddress string     // the P2PKH address required to spend the txn output (extracted from the script code)
}

// UTXOIterator is used to iterate over UTXOs in the Account.
type UTXOIterator interface {
	hasNext() bool
	getNext() UTXO
}

// NewTxn is a new Dogecoin Transaction being created by Gigawallet.
type NewTxn struct {
	TxnHex       string
	TotalIn      CoinAmount
	PayAmount    CoinAmount
	FeeAmount    CoinAmount
	ChangeAmount CoinAmount
}

// DecodedTxn is decoded from transaction hex data by L1/Core.
type DecodedTxn struct {
}

// Invoice is a request for payment created by Gigawallet.
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

// CalcTotal sums up the Items listed on the Invoice.
func (i *Invoice) CalcTotal() CoinAmount {
	total := ZeroCoins
	for _, item := range i.Items {
		total = total.Add(decimal.NewFromInt(int64(item.Quantity)).Mul(item.Price))
	}
	return total
}

type Item struct {
	Name      string          `json:"name"`
	Price     decimal.Decimal `json:"price"`
	Quantity  int             `json:"quantity"`
	ImageLink string          `json:"image_link"`
}
