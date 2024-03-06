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
	if res.StatusCode != 200 {
		return fmt.Errorf("json-rpc status code: %s", res.Status)
	}
	// cannot use json.NewDecoder: "The decoder introduces its own buffering
	// and may read data from r beyond the JSON values requested."
	var rpcres rpcResponse
	err = json.Unmarshal(res_bytes, &rpcres)
	if err != nil {
		return fmt.Errorf("json-rpc unmarshal response: %v", err)
	}
	if rpcres.Id != body.Id {
		return fmt.Errorf("json-rpc wrong ID returned: %v vs %v", rpcres.Id, body.Id)
	}
	if rpcres.Error != nil {
		return fmt.Errorf("json-rpc error returned: %v", rpcres.Error)
	}
	if rpcres.Result == nil {
		return fmt.Errorf("json-rpc missing result")
	}
	err = json.Unmarshal(*rpcres.Result, result)
	if err != nil {
		return fmt.Errorf("json-rpc unmarshal result: %v | %v", err, string(*rpcres.Result))
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

func (l *L1CoreRPC) GetTransaction(txnHash string) (txn giga.RawTxn, err error) {
	decode := true // to get back JSON rather than HEX
	err = l.request("getrawtransaction", []any{txnHash, decode}, &txn)
	return
}

func (l *L1CoreRPC) Send(txnHex string) (txid string, err error) {
	txn, err := doge.HexDecode(txnHex)
	if err != nil {
		return "", fmt.Errorf("sendrawtransaction: could not decode txnHex")
	}
	err = l.request("sendrawtransaction", []any{txnHex}, &txid)
	if len(txid) != 64 || !doge.IsValidHex(txid) {
		return "", fmt.Errorf("sendrawtransaction: did not return txid")
	}
	hash := doge.TxHashHex(txn)
	if txid != hash {
		log.Printf("[!] sendrawtransaction: did not return the expected txid: %s vs %s", txid, hash)
	}
	return
}

type estimatesmartfeeError struct {
	Str string `json:"str"`
}
type estimatesmartfeeResponse struct {
	FeeRate int64                   `json:"feerate"`
	Blocks  int64                   `json:"blocks"`
	Errors  []estimatesmartfeeError `json:"errors"`
}

func (l L1CoreRPC) EstimateFee(confirmTarget int) (feePerKB giga.CoinAmount, err error) {
	var res estimatesmartfeeResponse
	err = l.request("estimatesmartfee", []any{confirmTarget}, &res)
	if len(res.Errors) > 0 {
		return giga.ZeroCoins, errors.New(res.Errors[0].Str)
	}
	if res.FeeRate < 0 {
		return giga.ZeroCoins, errors.New("fee-rate is negative")
	}
	feePerKB = decimal.NewFromInt(res.FeeRate)
	return
}
