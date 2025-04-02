package giga

import (
	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	"github.com/shopspring/decimal"
)

/* dogecoin.go contains types for interfacing with
 * dogecoin layer 1, via core or libdogecoin. Things
 * here may be cantidates for moving into go-libdogecoin
 */

// L1 represents access to Dogecoin's L1 functionality.
//
// The general idea is that this will eventually be provided by a
// Go binding for the libdogecoin project, however to begin with
// will be implemented via RPC/ZMQ comms to the Dogecoin Core APIs.
type L1 interface {
	MakeAddress(isTestNet bool) (Address, Privkey, error)
	MakeChildAddress(privkey Privkey, addressIndex uint32, isInternal bool) (Address, error)
	MakeTransaction(inputs []UTXO, outputs []NewTxOut, fee CoinAmount, change Address, private_key Privkey) (NewTxn, error)
	DecodeTransaction(txnHex string) (RawTxn, error)
	GetBlock(blockHash string) (RpcBlock, error)
	GetBlockHex(blockHash string) (string, error)
	GetBlockHeader(blockHash string) (RpcBlockHeader, error)
	GetBlockHash(height int64) (string, error)
	GetBestBlockHash() (string, error)
	GetBlockCount() (int64, error)
	GetBlockchainInfo() (RpcBlockchainInfo, error)
	GetTransaction(txnHash string) (RawTxn, error)
	Send(txnHex string) (txid string, err error)
	EstimateFee(confirmTarget int) (feePerKB CoinAmount, err error)
	TestMempoolAccept(tx string, maxFeeRate string) (MempoolAccept, error)
	GetTxOut(txid string, vout uint32, include_mempool bool) (GetTxOut, error) // gettxout
}

type Address = doge.Address // Dogecoin address (base-58 public key hash aka PKH)
type Privkey string         // Extended Private Key for HD Wallet
type CoinAmount = decimal.Decimal
type ScriptType = doge.ScriptType

const OneCoin_64 = 100_000_000           // 1 DOGE
const TxnDustLimit_64 = OneCoin_64 / 100 // 0.01 DOGE
const NumKoinuDigits = 8                 // Maximum Koinu digits (after decimal point)

var ZeroCoins = decimal.NewFromInt(0)                           // 0 DOGE
var OneCoin = decimal.NewFromInt(1)                             // 1.0 DOGE
var TxnRecommendedMinFee = OneCoin.Div(decimal.NewFromInt(100)) // 0.01 DOGE (RECOMMENDED_MIN_TX_FEE in Core)
var TxnRecommendedMaxFee = OneCoin                              // 1 DOGE
var TxnFeePerKB = OneCoin.Div(decimal.NewFromInt(100))          // 0.01 DOGE
var TxnFeePerByte = TxnFeePerKB.Div(decimal.NewFromInt(1000))   // since Core version 1.14.5
var TxnDustLimit = OneCoin.Div(decimal.NewFromInt(100))         // 0.01 DOGE

// A new transaction (hex) from libdogecoin.
type NewTxn struct {
	TxnHex       string     // Transaction in Hexadecimal format.
	TotalIn      CoinAmount // Sum of all inputs (UTXOs) spent by the transaction.
	TotalOut     CoinAmount // Sum of all outputs (NewTxOuts) i.e. total amount paid (excludes fee)
	FeeAmount    CoinAmount // Fee paid by the transaction
	ChangeAmount CoinAmount // Change returned to wallet (excess input)
}

// NewTxOut is an output from a new Txn, i.e. creates a new UTXO.
type NewTxOut struct {
	ScriptType    ScriptType // 'p2pkh' etc, see ScriptType constants
	Amount        CoinAmount // Amount of Dogecoin to pay to the PayTo address
	ScriptAddress Address    // Dogecoin P2PKH Address to receive the funds
}

// Decode the 'Type' from Core RPC to our ScriptType enum.
// Core RPC uses completely different names, just to confuse everyone.
// See: standard.cpp line 24 `GetTxnOutputType` in Core.
func DecodeCoreRPCScriptType(coreRpcType string) ScriptType {
	switch coreRpcType {
	case "nonstandard":
		return doge.ScriptTypeCustom
	case "pubkey":
		return doge.ScriptTypeP2PK
	case "pubkeyhash":
		return doge.ScriptTypeP2PKH
	case "scripthash":
		return doge.ScriptTypeP2SH
	case "multisig":
		return doge.ScriptTypeMultiSig
	case "nulldata":
		return doge.ScriptTypeNullData
	case "witness_v0_keyhash":
		return doge.ScriptTypeP2PKHW
	case "witness_v0_scripthash":
		return doge.ScriptTypeP2SHW
	default:
		return doge.ScriptTypeCustom
	}
}

// RawTxn is decoded from transaction hex data by L1/Core.
// Derived from the `decoderawtransaction` Core API.
// Backgrounder: https://btcinformation.org/en/developer-guide#transactions
type RawTxn struct {
	TxID     string       `json:"txid"`     // The transaction id
	Hash     string       `json:"hash"`     // The transaction hash (differs from txid for witness transactions)
	Size     int64        `json:"size"`     // The transaction size
	VSize    int64        `json:"vsize"`    // The virtual transaction size (differs from size for witness transactions)
	Version  int64        `json:"version"`  // The version
	LockTime int64        `json:"locktime"` // The lock time
	VIn      []RawTxnVIn  `json:"vin"`      // Array of transaction inputs (UTXOs to spend)
	VOut     []RawTxnVOut `json:"vout"`     // Array of transaction outputs (UTXOs to create)
}
type RawTxnVIn struct {
	TxID        string          `json:"txid"`        // The transaction id (UTXO)
	VOut        int             `json:"vout"`        // The output number (UTXO)
	ScriptSig   RawTxnScriptSig `json:"scriptSig"`   // The "signature script" (solution to the UTXO "pubkey script")
	TxInWitness []string        `json:"txinwitness"` // Array of hex-encoded witness data (if any)
	Sequence    int64           `json:"sequence"`    // The script sequence number
}
type RawTxnScriptSig struct {
	Asm string `json:"asm"` // The script disassembly
	Hex string `json:"hex"` // The script hex
}
type RawTxnVOut struct {
	Value        decimal.Decimal    `json:"value"`        // The value in DOGE (an exact decimal number)
	N            int                `json:"n"`            // The output number (VOut when spending)
	ScriptPubKey RawTxnScriptPubKey `json:"scriptPubKey"` // The "pubkey script" (conditions for spending this output)
}
type RawTxnScriptPubKey struct {
	Asm       string   `json:"asm"`       // The script disassembly
	Hex       string   `json:"hex"`       // The script hex
	ReqSigs   int64    `json:"reqSigs"`   // Number of required signatures
	Type      string   `json:"type"`      // Core RPC Script Type (see DecodeCoreRPCScriptType) NB. does NOT match our ScriptType enum!
	Addresses []string `json:"addresses"` // Array of dogecoin addresses accepted by the script
}

// RpcBlock is decoded from block hex data by L1/Core.
// Derived from the `getblock` Core API.
// https://developer.bitcoin.org/reference/rpc/getblock.html
type RpcBlock struct {
	Hash              string          `json:"hash"`              // (string) the block hash (same as provided) (hex)
	Confirmations     int64           `json:"confirmations"`     // (numeric) The number of confirmations, or -1 if the block is not on the main chain
	Size              int             `json:"size"`              // (numeric) The block size
	StrippedSize      int             `json:"strippedsize"`      // (numeric) The block size excluding witness data
	Weight            int             `json:"weight"`            // (numeric) The block weight as defined in BIP 141
	Height            int64           `json:"height"`            // (numeric) The block height or index
	Version           int             `json:"version"`           // (numeric) The block version
	VersionHex        string          `json:"versionHex"`        // (string) The block version formatted in hexadecimal
	MerkleRoot        string          `json:"merkleroot"`        // (string) The merkle root (hex)
	Tx                []string        `json:"tx"`                // (json array) The transaction ids
	Time              int             `json:"time"`              // (numeric) The block time in seconds since UNIX epoch (Jan 1 1970 GMT)
	MedianTime        int             `json:"mediantime"`        // (numeric) The median block time in seconds since UNIX epoch (Jan 1 1970 GMT)
	Nonce             int             `json:"nonce"`             // (numeric) The nonce
	Bits              string          `json:"bits"`              // (string) The bits
	Difficulty        decimal.Decimal `json:"difficulty"`        // (numeric) The difficulty
	ChainWork         string          `json:"chainwork"`         // (string) Expected number of hashes required to produce the chain up to this block (hex)
	PreviousBlockHash string          `json:"previousblockhash"` // (string) The hash of the previous block (hex)
	NextBlockHash     string          `json:"nextblockhash"`     // (string) The hash of the next block (hex)
}

// RpcBlockHeader from Core includes on-chain status (Confirmations = -1 means on a fork)
// Derived from the `getblockheader` Core API.
// https://developer.bitcoin.org/reference/rpc/getblockheader.html
type RpcBlockHeader struct {
	Hash              string          `json:"hash"`              // (string) the block hash (same as provided) (hex)
	Confirmations     int64           `json:"confirmations"`     // (numeric) The number of confirmations, or -1 if the block is not on the main chain
	Height            int64           `json:"height"`            // (numeric) The block height or index
	Version           int             `json:"version"`           // (numeric) The block version
	VersionHex        string          `json:"versionHex"`        // (string) The block version formatted in hexadecimal
	MerkleRoot        string          `json:"merkleroot"`        // (string) The merkle root (hex)
	Time              int             `json:"time"`              // (numeric) The block time in seconds since UNIX epoch (Jan 1 1970 GMT)
	MedianTime        int             `json:"mediantime"`        // (numeric) The median block time in seconds since UNIX epoch (Jan 1 1970 GMT)
	Nonce             int             `json:"nonce"`             // (numeric) The nonce
	Bits              string          `json:"bits"`              // (string) The bits
	Difficulty        decimal.Decimal `json:"difficulty"`        // (numeric) The difficulty
	ChainWork         string          `json:"chainwork"`         // (string) Expected number of hashes required to produce the chain up to this block (hex)
	PreviousBlockHash string          `json:"previousblockhash"` // (string) The hash of the previous block (hex)
	NextBlockHash     string          `json:"nextblockhash"`     // (string) The hash of the next block (hex)
}

// RpcBlockchainInfo from Core
type RpcBlockchainInfo struct {
	Chain                string  `json:"chain"`                // (string) current network name (main, test, regtest)
	Blocks               int64   `json:"blocks"`               // (numeric) the height of the most-work fully-validated chain. The genesis block has height 0
	Headers              int64   `json:"headers"`              // (numeric) the current number of headers we have validated
	BestBlockHash        string  `json:"bestblockhash"`        // (string) the hash of the currently best block
	Difficulty           float64 `json:"difficulty"`           // (numeric) the current difficulty
	MedianTime           int64   `json:"mediantime"`           // (numeric) median time for the current best block
	VerificationProgress float64 `json:"verificationprogress"` // (numeric) estimate of verification progress [0..1]
	InitialBlockDownload bool    `json:"initialblockdownload"` // (boolean) (debug information) estimate of whether this node is in Initial Block Download mode
	ChainWord            string  `json:"chainwork"`            // (string) total amount of work in active chain, in hexadecimal
	SizeOnDisk           int64   `json:"size_on_disk"`         // (numeric) the estimated size of the block and undo files on disk
	Pruned               bool    `json:"pruned"`               // (boolean) if the blocks are subject to pruning
	PruneHeight          int64   `json:"pruneheight"`          // (numeric) lowest-height complete block stored (only present if pruning is enabled)
	AutomaticPruning     bool    `json:"automatic_pruning"`    // (boolean) whether automatic pruning is enabled (only present if pruning is enabled)
	PruneTargetSize      int64   `json:"prune_target_size"`    // (numeric) the target size used by pruning (only present if automatic pruning is enabled)

}

// MempoolAccept is the response from testmempoolaccept Core RPC
type MempoolAccept struct {
	TxID         string      `json:"txid"`          // The transaction hash in hex
	Allowed      bool        `json:"allowed"`       // If the mempool allows this tx to be inserted
	VSize        int64       `json:"vsize"`         // Virtual transaction size as defined in BIP 141. This is different from actual serialized size for witness transactions as witness data is discounted (only present when 'allowed' is true)
	Fees         MempoolFees `json:"fees"`          // Transaction fees (only present if 'allowed' is true)
	RejectReason string      `json:"reject-reason"` // Rejection string (only present when 'allowed' is false)
}

type MempoolFees struct {
	Base string `json:"base"` // Transaction fee in Doge, DECIMAL
}

// GetTxOut is the result of a `gettxout` Core RPC
//
//	{
//	  "bestblock": "f21a25e6980c00d5bb72b58fc490e52328b1bc78ec046ea5d65b8199fa624f06",
//	  "confirmations": 1,
//	  "value": 171.30751031,
//	  "scriptPubKey": {
//	    "asm": "OP_DUP OP_HASH160 44159c14228e731c5c2a247a1c25119a264f558e OP_EQUALVERIFY OP_CHECKSIG",
//	    "hex": "76a91444159c14228e731c5c2a247a1c25119a264f558e88ac",
//	    "reqSigs": 1,
//	    "type": "pubkeyhash",
//	    "addresses": [
//	      "DBM6NmcNaL7HB7wNVNewbPETpq6ZJGe4Yr"
//	    ]
//	  },
//	  "version": 1,
//	  "coinbase": false
//	}
type GetTxOut struct {
	BestBlock     string             `json:"bestblock"`     // Hash of the block at the tip of the chain (hex)
	Confirmations int64              `json:"confirmations"` // Number of confirmations (blocks)
	Value         RawNumber          `json:"value"`         // Transaction value in BTC, DECIMAL string
	ScriptPubKey  RawTxnScriptPubKey `json:"scriptPubKey"`  // Output script
	Version       int                `json:"version"`       // Transaction version
	Coinbase      bool               `json:"coinbase"`      // Coinbase or not
}

// RawNumber parses a JSON number as a string, to preserve accuracy
type RawNumber struct {
	n string
}

// UnmarshalJSON implments json.Unmarshaler
func (val *RawNumber) UnmarshalJSON(data []byte) error {
	val.n = string(data)
	return nil
}
