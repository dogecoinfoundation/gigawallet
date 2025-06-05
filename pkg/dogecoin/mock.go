package dogecoin

import (
	"encoding/hex"
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/shopspring/decimal"
)

// interface guard ensures L1Mock implements giga.L1
var _ giga.L1 = L1Mock{}

// NewL1Mock returns a mocked giga.L1 implementor
func NewL1Mock(config giga.Config) (L1Mock, error) {
	return L1Mock{}, nil
}

type L1Mock struct {
	RawBlockHeader string // for GetRawBlockHeader
}

func (l L1Mock) MakeAddress(isTestNet bool) (giga.Address, giga.Privkey, error) {
	return "mockAddress", "mockPrivkey", nil
}

func (l L1Mock) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	return "mockChildAddress", nil
}

func (l L1Mock) MakeTransaction(inputs []giga.UTXO, outputs []giga.NewTxOut, fee giga.CoinAmount, change giga.Address, private_key giga.Privkey) (giga.NewTxn, error) {
	return giga.NewTxn{}, fmt.Errorf("not implemented")
}

func (l L1Mock) DecodeTransaction(txnHex string) (giga.RawTxn, error) {
	return giga.RawTxn{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlock(blockHash string) (txn giga.RpcBlock, err error) {
	return giga.RpcBlock{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlockHex(blockHash string) (hex string, err error) {
	return "", fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlockHeader(blockHash string) (txn giga.RpcBlockHeader, err error) {
	return giga.RpcBlockHeader{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetRawBlockHeader(blockHash string) (bytes []byte, err error) {
	if l.RawBlockHeader != "" {
		bytes, err = hex.DecodeString(l.RawBlockHeader)
		return
	}
	return []byte{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlockHash(height int64) (hash string, err error) {
	return "", fmt.Errorf("not implemented")
}

func (l L1Mock) GetBestBlockHash() (blockHash string, err error) {
	return "FEED000000000000000000000000000000000000000000000000000000000000", nil
}

func (l L1Mock) GetBlockCount() (blockCount int64, err error) {
	return 100, nil
}

func (l L1Mock) GetBlockchainInfo() (info giga.RpcBlockchainInfo, err error) {
	return giga.RpcBlockchainInfo{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetTransaction(txnHash string) (txn giga.RawTxn, err error) {
	return giga.RawTxn{}, nil
}

func (l L1Mock) Send(txnHex string) (txid string, err error) {
	return "FEED000000000000000000000000000000000000000000000000000000000000", nil
}

func (l L1Mock) EstimateFee(confirmTarget int) (feePerKB giga.CoinAmount, err error) {
	return decimal.NewFromString("0.67891013") // example from Core
}
