package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// interface guard ensures L1CoreRPC implements giga.L1
var _ giga.L1 = L1CoreRPC{}

// NewDogecoinCoreRPC returns a giga.L1 implementor that uses dogecoin-core's RPC
func NewDogecoinCoreRPC(config giga.Config) (L1CoreRPC, error) {
	addr := fmt.Sprintf("http://%s:%d", config.Dogecoind[config.Gigawallet.Dogecoind].RPCHost, config.Dogecoind[config.Gigawallet.Dogecoind].RPCPort)
	user := config.Dogecoind[config.Gigawallet.Dogecoind].RPCUser
	pass := config.Dogecoind[config.Gigawallet.Dogecoind].RPCPass
	return L1CoreRPC{addr, user, pass, 1}, nil
}

type L1CoreRPC struct {
	url  string
	user string
	pass string
	id   uint64
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
	body := rpcRequest{
		Method: method,
		Params: params,
		Id:     l.id,
	}
	l.id += 1 // each request should use a unique ID
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
		return fmt.Errorf("json-rpc unmarshal result: %v", err)
	}
	return nil
}

func (l L1CoreRPC) MakeAddress() (giga.Address, giga.Privkey, error) {
	return "", "", fmt.Errorf("not implemented")
}

func (l L1CoreRPC) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	return "", fmt.Errorf("not implemented")
}

func (l L1CoreRPC) MakeTransaction(amount giga.CoinAmount, UTXOs []giga.UTXO, payTo giga.Address, fee giga.CoinAmount, change giga.Address, private_key_wif giga.Privkey) (giga.NewTxn, error) {
	return giga.NewTxn{}, fmt.Errorf("not implemented")
}

func (l L1CoreRPC) DecodeTransaction(txn_hex string) (txn giga.RawTxn, err error) {
	err = l.request("decoderawtransaction", []any{txn_hex}, &txn)
	return
}

func (l L1CoreRPC) Send(txn giga.NewTxn) error {
	return fmt.Errorf("not implemented")
}
