package dogecoin

import (
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	"github.com/dogeorg/go-libdogecoin"
)

// Signature hash types/flags from libdogecoin
const (
	SIGHASH_ALL          = 1
	SIGHASH_NONE         = 2
	SIGHASH_SINGLE       = 3
	SIGHASH_ANYONECANPAY = 0x80 // flag
)

// interface guard ensures L1Libdogecoin implements giga.L1
var _ giga.L1 = L1Libdogecoin{}

// NewL1Libdogecoin returns a giga.L1 implementor that uses libdogecoin
// Allows (non-implemented) functions to delegate to another L1 implementation.
func NewL1Libdogecoin(config giga.Config, fallback giga.L1) (L1Libdogecoin, error) {
	return L1Libdogecoin{fallback: fallback}, nil
}

type L1Libdogecoin struct {
	fallback giga.L1
}

func (l L1Libdogecoin) MakeAddress() (giga.Address, giga.Privkey, error) {
	libdogecoin.W_context_start()
	priv, pub := libdogecoin.W_generate_hd_master_pub_keypair(false)
	libdogecoin.W_context_stop()
	return giga.Address(pub), giga.Privkey(priv), nil
}

func (l L1Libdogecoin) MakeChildAddress(privkey giga.Privkey, addressIndex uint32, isInternal bool) (giga.Address, error) {
	libdogecoin.W_context_start()
	// this API is a bit odd: it returns the "extended public key"
	// which you can think of as a coordinate in the HD Wallet key-space.
	hd_node := libdogecoin.W_get_derived_hd_address(string(privkey), 0, isInternal, addressIndex, false)
	// derive the dogecoin address (hash) from the extended public-key
	pub := libdogecoin.W_generate_derived_hd_pub_key(hd_node)
	libdogecoin.W_context_stop()
	return giga.Address(pub), nil
}

func (l L1Libdogecoin) MakeTransaction(amount giga.CoinAmount, UTXOs []giga.UTXO, payTo giga.Address, fee giga.CoinAmount, change giga.Address, private_key_wif giga.Privkey) (giga.NewTxn, error) {
	libdogecoin.W_context_start()
	defer libdogecoin.W_context_stop()

	// validate transaction amounts
	if len(UTXOs) < 1 {
		return giga.NewTxn{}, fmt.Errorf("cannot make a txn with zero UTXOs")
	}
	totalIn := giga.ZeroCoins
	for _, UTXO := range UTXOs {
		totalIn = totalIn.Add(UTXO.Value)
	}
	minRequired := amount.Add(fee)
	if totalIn.LessThan(minRequired) {
		return giga.NewTxn{}, fmt.Errorf("UTXOs do not hold enough value to pay amount plus fee: %s vs %s", totalIn.String(), minRequired.String())
	}

	// create the transaction
	tx := libdogecoin.W_start_transaction()
	defer libdogecoin.W_clear_transaction(tx)

	// add the UTXOs to spend in the transaction
	for _, UTXO := range UTXOs {
		if libdogecoin.W_add_utxo(tx, UTXO.TxnID, UTXO.VOut) != 1 {
			return giga.NewTxn{}, fmt.Errorf("libdogecoin error adding UTXO: %v", UTXO)
		}
	}

	// add output: P2PKH to the payTo Address
	if libdogecoin.W_add_output(tx, string(payTo), amount.String()) != 1 {
		return giga.NewTxn{}, fmt.Errorf("libdogecoin error adding payTo output")
	}

	// finalize the transaction: adds a change output if necessary
	// the first address (destination_address) is only used to determine main-net or test-net.
	// the final argument is the change_address which will be used to add a txn output if there is any change.
	tx_hex := libdogecoin.W_finalize_transaction(tx, string(payTo), fee.String(), totalIn.String(), string(change))
	if tx_hex == "" {
		return giga.NewTxn{}, fmt.Errorf("libdogecoin error finalizing transaction")
	}

	// FIXME: safer to extract this from the transaction output added by libdogecoin (if any)
	change_amt := totalIn.Sub(amount).Sub(fee)

	// we have the payer's private key in WIF format.
	// generate the payer's public P2PKH Address from the private key.
	p2pkh_pub := libdogecoin.W_generate_derived_hd_pub_key(string(private_key_wif))
	if p2pkh_pub == "" {
		return giga.NewTxn{}, fmt.Errorf("libdogecoin error generating pubkey for privkey")
	}

	// sign the transaction: sign each input UTXO with our public and private key
	// note: assumes all UTXOs were created with standard P2PKH script (no Multisig etc)
	// and the same private key (FIXME: won't be true for HD-Wallet UTXOs)
	if libdogecoin.W_sign_transaction(tx, p2pkh_pub, string(private_key_wif)) != 1 {
		return giga.NewTxn{}, fmt.Errorf("libdogecoin failed to sign transaction")
	}

	// we might need to sign each input separately (if each UTXO has a different pay-to Address,
	//                                      don't we need a different private key for each one?)
	// for n := range UTXOs {
	// 	tx_hex = libdogecoin.W_sign_raw_transaction(n, tx_hex, script_pubkey, SIGHASH_ALL, private_key_wif)
	// }

	return giga.NewTxn{TxnHex: tx_hex, TotalIn: totalIn, PayAmount: amount, FeeAmount: fee, ChangeAmount: change_amt}, nil
}

func (l L1Libdogecoin) DecodeTransaction(txnHex string) (giga.RawTxn, error) {
	if l.fallback != nil {
		return l.fallback.DecodeTransaction(txnHex)
	}
	return giga.RawTxn{}, fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) GetBlock(blockHash string) (txn giga.RpcBlock, err error) {
	if l.fallback != nil {
		return l.fallback.GetBlock(blockHash)
	}
	return giga.RpcBlock{}, fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) GetBlockHeader(blockHash string) (txn giga.RpcBlockHeader, err error) {
	if l.fallback != nil {
		return l.fallback.GetBlockHeader(blockHash)
	}
	return giga.RpcBlockHeader{}, fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) GetBlockHash(height int64) (hash string, err error) {
	if l.fallback != nil {
		return l.fallback.GetBlockHash(height)
	}
	return "", fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) GetBestBlockHash() (blockHash string, err error) {
	if l.fallback != nil {
		return l.fallback.GetBestBlockHash()
	}
	return "", fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) GetTransaction(txnHash string) (txn giga.RawTxn, err error) {
	if l.fallback != nil {
		return l.fallback.GetTransaction(txnHash)
	}
	return giga.RawTxn{}, fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) Send(txn giga.NewTxn) error {
	if l.fallback != nil {
		return l.fallback.Send(txn)
	}
	return fmt.Errorf("not implemented")
}
