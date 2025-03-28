package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	"github.com/shopspring/decimal"
)

// interface guard ensures L1CoreRPC implements giga.L1
var _ giga.L1 = &L1CoreRPC{}

// NewDogecoinCoreRPC returns a giga.L1 implementor that uses dogecoin-core's RPC
func NewDogecoinCoreRPC(config giga.Config) (*L1CoreRPC, error) {
	addr := fmt.Sprintf("http://%s:%d", config.Core.RPCHost, config.Core.RPCPort)
	user := config.Core.RPCUser
	pass := config.Core.RPCPass
	return &L1CoreRPC{url: addr, user: user, pass: pass}, nil
}

type L1CoreRPC struct {
	url  string
	user string
	pass string
	id   atomic.Uint64 // next unique request id
}

type rpcRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	Id     uint64 `json:"id"`
}
type rpcResponse struct {
	Id     uint64           `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  any              `json:"error"`
}

func (l *L1CoreRPC) request(method string, params []any, result any) error {
	id := l.id.Add(1) // each request should use a unique ID
	body := rpcRequest{
		Method: method,
		Params: params,
		Id:     id,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("json-rpc marshal request: %v", err)
	}
	req, err := http.NewRequest("POST", l.url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("json-rpc request: %v", err)
	}
	req.SetBasicAuth(l.user, l.pass)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("json-rpc transport: %v", err)
	}
	// we MUST read all of res.Body and call res.Close,
	// otherwise the underlying connection cannot be re-used.
	defer res.Body.Close()
	res_bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("json-rpc read response: %v", err)
	}
	// check for error response
	if res.StatusCode != 200 {
		return fmt.Errorf("json-rpc error status: %v", res.StatusCode)
	}
	// cannot use json.NewDecoder: "The decoder introduces its own buffering
	// and may read data from r beyond the JSON values requested."
	var rpcres rpcResponse
	err = json.Unmarshal(res_bytes, &rpcres)
	if err != nil {
		return fmt.Errorf("json-rpc unmarshal response: %v | %v", err, string(res_bytes))
	}
	if rpcres.Id != body.Id {
		return fmt.Errorf("json-rpc wrong ID returned: %v vs %v", rpcres.Id, body.Id)
	}
	if rpcres.Error != nil {
		enc, err := json.Marshal(rpcres.Error)
		if err == nil {
			return fmt.Errorf("json-rpc: error from Core Node: %v", string(enc))
		} else {
			return fmt.Errorf("json-rpc: error from Core Node: %v", rpcres.Error)
		}
	}
	if rpcres.Result == nil {
		return fmt.Errorf("json-rpc no result or error was returned")
	}
	err = json.Unmarshal(*rpcres.Result, result)
	if err != nil {
		return fmt.Errorf("json-rpc unmarshal error: %v | %v", err, string(*rpcres.Result))
	}
	return nil
}

func (l *L1CoreRPC) MakeAddress(isTestNet bool) (giga.Address, giga.Privkey, error) {
	return "", "", fmt.Errorf("not implemented")
}

func (l *L1CoreRPC) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	return "", fmt.Errorf("not implemented")
}

func (l *L1CoreRPC) MakeTransaction(inputs []giga.UTXO, outputs []giga.NewTxOut, fee giga.CoinAmount, change giga.Address, private_key giga.Privkey) (giga.NewTxn, error) {
	return giga.NewTxn{}, fmt.Errorf("not implemented")
}

func (l *L1CoreRPC) DecodeTransaction(txn_hex string) (txn giga.RawTxn, err error) {
	err = l.request("decoderawtransaction", []any{txn_hex}, &txn)
	return
}

func (l *L1CoreRPC) GetBlock(blockHash string) (txn giga.RpcBlock, err error) {
	decode := true // to get back JSON rather than HEX
	err = l.request("getblock", []any{blockHash, decode}, &txn)
	return
}

func (l *L1CoreRPC) GetBlockHex(blockHash string) (hex string, err error) {
	decode := false // to get back HEX
	err = l.request("getblock", []any{blockHash, decode}, &hex)
	return
}

func (l *L1CoreRPC) GetBlockHeader(blockHash string) (txn giga.RpcBlockHeader, err error) {
	decode := true // to get back JSON rather than HEX
	err = l.request("getblockheader", []any{blockHash, decode}, &txn)
	return
}

func (l *L1CoreRPC) GetBlockHash(height int64) (hash string, err error) {
	err = l.request("getblockhash", []any{height}, &hash)
	return
}

func (l *L1CoreRPC) GetBestBlockHash() (blockHash string, err error) {
	err = l.request("getbestblockhash", []any{}, &blockHash)
	return
}

func (l *L1CoreRPC) GetBlockCount() (blockCount int64, err error) {
	err = l.request("getblockcount", []any{}, &blockCount)
	return
}

func (l *L1CoreRPC) GetBlockchainInfo() (info giga.RpcBlockchainInfo, err error) {
	err = l.request("getblockchaininfo", []any{}, &info)
	return
}

func (l *L1CoreRPC) GetTransaction(txnHash string) (txn giga.RawTxn, err error) {
	decode := true // to get back JSON rather than HEX
	err = l.request("getrawtransaction", []any{txnHash, decode}, &txn)
	return
}

func (l *L1CoreRPC) Send(txnHex string) (txid string, err error) {
	log.Printf("SEND Tx: %v", txnHex)
	txn, err := doge.HexDecode(txnHex)
	if err != nil {
		return "", fmt.Errorf("sendrawtransaction: could not decode txnHex")
	}
	err = l.request("sendrawtransaction", []any{txnHex}, &txid)
	if err != nil {
		return "", fmt.Errorf("sendrawtransaction: %v", err)
	}
	if len(txid) != 64 || !doge.IsValidHex(txid) {
		if len(txid) < 1 {
			txid = "(nothing returned)"
		}
		return "", fmt.Errorf("sendrawtransaction: Core Node did not return txid: %v", txid)
	}
	hash := doge.TxHashHex(txn)
	if txid != hash {
		log.Printf("[!] sendrawtransaction: Core Node did not return the expected txid: %s (expecting %s)", txid, hash)
	}
	return
}

func (l *L1CoreRPC) EstimateFee(confirmTarget int) (feePerKB giga.CoinAmount, err error) {
	var res RawNumber
	err = l.request("estimatefee", []any{confirmTarget}, &res)
	if err != nil {
		return
	}
	feePerKB, err = decimal.NewFromString(res.n)
	if err != nil {
		return
	}
	if feePerKB.LessThan(decimal.Zero) {
		return giga.ZeroCoins, errors.New("fee-rate is negative")
	}
	return
}

func (l *L1CoreRPC) TestMempoolAccept(tx string, maxFeeRate string) (giga.MempoolAccept, error) {
	var res []giga.MempoolAccept
	txs := []string{tx}
	err := l.request("testmempoolaccept", []any{txs, maxFeeRate}, &res)
	if err != nil {
		return giga.MempoolAccept{}, fmt.Errorf("testmempoolaccept: %v", err)
	}
	if len(res) < 1 {
		return giga.MempoolAccept{}, fmt.Errorf("testmempoolaccept: no results")
	}
	return res[0], nil
}

func (l *L1CoreRPC) GetTxOut(txid string, vout uint32, include_mempool bool) (res giga.GetTxOut, err error) {
	err = l.request("gettxout", []any{txid, vout, include_mempool}, &res)
	if err != nil {
		return giga.GetTxOut{}, fmt.Errorf("gettxout: %v", err)
	}
	return
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
