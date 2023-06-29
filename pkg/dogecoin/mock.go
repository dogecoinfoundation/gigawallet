package dogecoin

import (
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

// interface guard ensures L1Mock implements giga.L1
var _ giga.L1 = L1Mock{}

// NewL1Mock returns a mocked giga.L1 implementor
func NewL1Mock(config giga.Config) (L1Mock, error) {
	return L1Mock{}, nil
}

type L1Mock struct{}

func (l L1Mock) MakeAddress() (giga.Address, giga.Privkey, error) {
	return "mockAddress", "mockPrivkey", nil
}

func (l L1Mock) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	return "mockChildAddress", nil
}

func (l L1Mock) MakeTransaction(amount giga.CoinAmount, UTXOs []giga.UTXO, payTo giga.Address, fee giga.CoinAmount, change giga.Address, private_key giga.Privkey) (giga.NewTxn, error) {
	return giga.NewTxn{}, fmt.Errorf("not implemented")
}

func (l L1Mock) DecodeTransaction(txnHex string) (giga.RawTxn, error) {
	return giga.RawTxn{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlock(blockHash string) (txn giga.RpcBlock, err error) {
	return giga.RpcBlock{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlockHeader(blockHash string) (txn giga.RpcBlockHeader, err error) {
	return giga.RpcBlockHeader{}, fmt.Errorf("not implemented")
}

func (l L1Mock) GetBlockHash(height int64) (hash string, err error) {
	return "", fmt.Errorf("not implemented")
}

func (l L1Mock) GetBestBlockHash() (blockHash string, err error) {
	return "", fmt.Errorf("not implemented")
}

func (l L1Mock) GetTransaction(txnHash string) (txn giga.RawTxn, err error) {
	return giga.RawTxn{}, fmt.Errorf("not implemented")
}

func (l L1Mock) Send(txn giga.NewTxn) error {
	return fmt.Errorf("not implemented")
}
