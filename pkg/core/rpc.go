package core

import (
	"fmt"
	"net/rpc"
	"net/rpc/jsonrpc"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// interface guard ensures L1CoreRPC implements giga.L1
var _ giga.L1 = L1CoreRPC{}

// NewDogecoinCoreRPC returns a giga.L1 implementor that uses dogecoin-core's RPC
func NewDogecoinCoreRPC(config giga.Config) (L1CoreRPC, error) {
	// Connect to the dogecoin daemon
	addr := fmt.Sprintf("%s:%d", config.Dogecoind[config.Gigawallet.Dogecoind].Host, config.Dogecoind[config.Gigawallet.Dogecoind].RPCPort)
	fmt.Println("Dialing:", addr)
	c, err := jsonrpc.Dial("tcp", addr)
	if err != nil {
		return L1CoreRPC{}, err
	}
	fmt.Println("Dialed")

	return L1CoreRPC{c}, nil
}

type L1CoreRPC struct {
	client *rpc.Client
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

type DecodeRawTransactionArgs struct {
	Hex string `json:"hexstring"`
}

func (l L1CoreRPC) DecodeTransaction(txn_hex string) (txn giga.RawTxn, err error) {
	args := DecodeRawTransactionArgs{Hex: txn_hex}
	err = l.client.Call("decoderawtransaction", &args, &txn)
	return
}

func (l L1CoreRPC) Send(txn giga.NewTxn) error {
	return fmt.Errorf("not implemented")
}
