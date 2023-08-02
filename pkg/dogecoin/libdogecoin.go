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

func (l L1Libdogecoin) MakeAddress(isTestNet bool) (giga.Address, giga.Privkey, error) {
	libdogecoin.W_context_start()
	priv, pub := libdogecoin.W_generate_hd_master_pub_keypair(isTestNet)
	if priv == "" || pub == "" {
		return "", "", giga.NewErr(giga.L1Error, "cannot generate_hd_master_pub_keypair")
	}
	libdogecoin.W_context_stop()
	return giga.Address(pub), giga.Privkey(priv), nil
}

func (l L1Libdogecoin) MakeChildAddress(privkey giga.Privkey, keyIndex uint32, isInternal bool) (giga.Address, error) {
	libdogecoin.W_context_start()
	// this API is a bit odd: it returns the "extended public key"
	// which you can think of as a coordinate in the HD Wallet key-space.
	hd_node_pub := libdogecoin.W_get_derived_hd_address(string(privkey), 0, isInternal, keyIndex, false)
	if hd_node_pub == "" {
		return "", giga.NewErr(giga.L1Error, "cannot get_derived_hd_address")
	}
	// derive the dogecoin address (hash) from the extended public-key
	pkh := libdogecoin.W_generate_derived_hd_pub_key(hd_node_pub)
	if pkh == "" {
		return "", giga.NewErr(giga.L1Error, "cannot generate_derived_hd_pub_key")
	}
	libdogecoin.W_context_stop()
	return giga.Address(pkh), nil
}

func (l L1Libdogecoin) MakeTransaction(inputs []giga.UTXO, outputs []giga.NewTxOut, fee giga.CoinAmount, change giga.Address, private_key_wif giga.Privkey) (giga.NewTxn, error) {
	libdogecoin.W_context_start()
	defer libdogecoin.W_context_stop()

	// validate transaction amounts
	if len(inputs) < 1 || len(outputs) < 1 {
		return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot make a txn with zero inputs or zero outputs")
	}
	totalIn := giga.ZeroCoins
	for _, utxo := range inputs {
		totalIn = totalIn.Add(utxo.Value)
	}
	totalOut := giga.ZeroCoins
	for _, out := range outputs {
		totalOut = totalOut.Add(out.Amount)
	}
	outPlusFee := totalOut.Add(fee)
	if totalIn.LessThan(outPlusFee) {
		return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "inputs do not hold enough value to pay outputs plus fee: %s vs %s", totalIn.String(), outPlusFee.String())
	}
	if totalIn.GreaterThan(outPlusFee) {
		return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "total inputs exceed total outputs plus fee: %s vs %s", totalIn.String(), outPlusFee.String())
	}

	// create the transaction
	tx := libdogecoin.W_start_transaction()
	defer libdogecoin.W_clear_transaction(tx)

	// add transaction inputs: UTXOs to spend.
	for _, utxo := range inputs {
		if libdogecoin.W_add_utxo(tx, utxo.TxID, utxo.VOut) != 1 {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot add transaction input: %v", utxo)
		}
	}

	// add transaction outputs: P2PKH paid to ScriptAddress.
	var anyOutputAddress string
	for _, out := range outputs {
		if libdogecoin.W_add_output(tx, string(out.ScriptAddress), out.Amount.String()) != 1 {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot add transaction output: %v", out)
		}
		anyOutputAddress = string(out.ScriptAddress)
	}

	// finalize the transaction: adds a change output if necessary.
	// the first address (destination_address) is only used to determine main-net or test-net.
	// the final argument is the change_address which will be used to add a txn output if there is any change.
	tx_hex := libdogecoin.W_finalize_transaction(tx, anyOutputAddress, fee.String(), totalIn.String(), string(change))
	if tx_hex == "" {
		return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot finalize_transaction")
	}

	// FIXME: safer to extract this from the transaction output added by libdogecoin (if any)
	change_amt := totalIn.Sub(totalOut).Sub(fee)

	// Sign the transaction: we need to sign each input UTXO separately,
	// because each one is generated from our HD Wallet with a different P2PKH Address.
	for n, utxo := range inputs {
		// Locate the HD Node in the HD Wallet for the Private Key at KeyIndex.
		// The PK should be the key for the ScriptAddress we extracted from the UTXO.
		hd_node_pk := libdogecoin.W_get_derived_hd_address(string(private_key_wif), 0, utxo.IsInternal, utxo.KeyIndex, true)
		if hd_node_pk == "" {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot get_derived_hd_address priv: %v", utxo)
		}
		hd_node_pub := libdogecoin.W_get_derived_hd_address(string(private_key_wif), 0, utxo.IsInternal, utxo.KeyIndex, false)
		if hd_node_pub == "" {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot get_derived_hd_address pub: %v", utxo)
		}

		// Problem 1:
		// Given HD PK: dgpv5Brh8HrjmwCZVn2c4d89qvaSRbtXVhncGKzFdkxevBD48jyZ2QFPBDC5kqyGcPDFeMAZyjMxFmFEnj4PM1LQZ7GicpjqKkFWMmY7jYYkMZo
		// How to get "privkey_wif" eg. ci5prbqz7jXyFPVWKkHhPq4a9N8Dag3TpeRfuqqC2Nfr7gSqx1fy

		// Problem 2:
		// Given HD Pub: dgub8vjpTyndL5rtnpgm6rk8xn82uAJAKppKYknTokMuiJG7go5FSAo8qWbjL88sShdBQLHn1xkAzEuqRRXPzwDJ7KNkune83STCYkXF4WqtdAc
		// How to get "utxo_scriptpubkey" eg. 76a914d8c43e6f68ca4ea1e9b93da2d1e3a95118fa4a7c88ac

		// Both of the above are required to call W_sign_raw_transaction !!

		// Generate the corresponding P2PKH Address for this PK.
		hd_p2pkh := libdogecoin.W_generate_derived_hd_pub_key(hd_node_pk)
		if hd_p2pkh == "" {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot generate_derived_hd_pub_key: %v", utxo)
		}
		// Verify we have the right PK for the UTXO ScriptAddress.
		if hd_p2pkh != string(utxo.ScriptAddress) {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "HD Private Key doesn't match UTXO ScriptAddress: %v", utxo)
		}

		// sign the Nth transaction input (i.e. generate the unlocking script)
		// [input_index, incoming_raw_tx string, script_hex string, sig_hash_type int, privkey string]
		// "the pubkey script in hexadecimal format (scripthex)"
		tx_hex = libdogecoin.W_sign_raw_transaction(n, tx_hex, hd_p2pkh, SIGHASH_ALL, hd_node_pk)
		if tx_hex == "" {
			return giga.NewTxn{}, giga.NewErr(giga.InvalidTxn, "cannot sign_raw_transaction: %v", utxo)
		}
	}

	return giga.NewTxn{TxnHex: tx_hex, TotalIn: totalIn, TotalOut: totalOut, FeeAmount: fee, ChangeAmount: change_amt}, nil
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

func (l L1Libdogecoin) GetBlockCount() (blockCount int64, err error) {
	if l.fallback != nil {
		return l.fallback.GetBlockCount()
	}
	return 0, fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) GetTransaction(txnHash string) (txn giga.RawTxn, err error) {
	if l.fallback != nil {
		return l.fallback.GetTransaction(txnHash)
	}
	return giga.RawTxn{}, fmt.Errorf("not implemented")
}

func (l L1Libdogecoin) Send(txnHex string) error {
	if l.fallback != nil {
		return l.fallback.Send(txnHex)
	}
	return fmt.Errorf("not implemented")
}
