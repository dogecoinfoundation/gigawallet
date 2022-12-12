package dogecoin

import (
	"fmt"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"

	"github.com/dogeorg/go-libdogecoin"
)

// interface guard ensures L1Libdogecoin implements giga.L1
var _ giga.L1 = L1Libdogecoin{}

// NewL1Libdogecoin returns a giga.L1 implementor that uses libdogecoin
func NewL1Libdogecoin(config giga.Config) (L1Libdogecoin, error) {
	return L1Libdogecoin{}, nil
}

type L1Libdogecoin struct{}

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

func (l L1Libdogecoin) MakeTransaction(amount giga.Koinu, UTXOs []giga.UTXO, payTo giga.Address, fee giga.Koinu, change giga.Address) (giga.Txn, error) {
	libdogecoin.W_context_start()
	defer libdogecoin.W_context_stop()

	// validate transaction amounts
	if len(UTXOs) < 1 {
		return giga.Txn{}, fmt.Errorf("cannot make a txn with zero UTXOs")
	}
	var totalIn giga.Koinu = 0
	for _, UTXO := range UTXOs {
		totalIn += UTXO.Value
	}
	minRequired := amount + fee
	if totalIn < minRequired {
		return giga.Txn{}, fmt.Errorf("UTXOs do not hold enough value to pay amount plus fee: %s vs %s", totalIn.ToCoinString(), minRequired.ToCoinString())
	}

	// create the transaction
	tx := libdogecoin.W_start_transaction()
	defer libdogecoin.W_clear_transaction(tx)

	// add the UTXOs to spend in the transaction
	for _, UTXO := range UTXOs {
		if libdogecoin.W_add_utxo(tx, UTXO.TxnID, UTXO.VOut) != 1 {
			return giga.Txn{}, fmt.Errorf("libdogecoin error adding UTXO: %v", UTXO)
		}
	}

	// add output: P2PKH for the payTo Address
	if libdogecoin.W_add_output(tx, string(payTo), amount.ToCoinString()) != 1 {
		return giga.Txn{}, fmt.Errorf("libdogecoin error adding payTo output")
	}

	// finalize the transaction: adds a change output if necessary
	// the first address (destination_address) is only used to determine main-net or test-net.
	// the final argument is the change_address which will be used to add a txn output if there is any change.
	tx_hex := libdogecoin.W_finalize_transaction(tx, string(payTo), fee.ToCoinString(), totalIn.ToCoinString(), string(change))
	if tx_hex == "" {
		return giga.Txn{}, fmt.Errorf("libdogecoin error finalizing transaction")
	}

	// sign the transaction inputs
	//

	return giga.Txn{TxnHex: tx_hex, InAmount: totalIn, PayAmount: amount, FeeAmount: fee, ChangeAmount: totalIn - amount - fee}, nil
}

func (l L1Libdogecoin) Send(txn giga.Txn) error {
	return nil
}
