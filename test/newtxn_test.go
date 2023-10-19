package test

import (
	"testing"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/shopspring/decimal"
)

func TestNewTxn(t *testing.T) {
	lib := newTestRig(t)
	acc := makeAccount(t, "Pepper", lib)

	// Setup: generate 10 UTXOs worth 0.5 - 50 doge each.
	var testUTXOs []giga.UTXO
	for vout := 0; vout < 10; vout++ {
		testUTXOs = append(testUTXOs, makeUTXO(t, vout, "2", &acc, lib))
	}

	// Setup: generate a couple of destination addresses.
	to_1, err := acc.NextChangeAddress(lib)
	if err != nil {
		t.Fatalf("NextChangeAddress: %v", err)
	}
	to_2, err := acc.NextChangeAddress(lib)
	if err != nil {
		t.Fatalf("NextChangeAddress: %v", err)
	}

	t.Run("CreateTxn with DeductFeePercent", func(t *testing.T) {
		// Pay to Multiple Addresses with percentage split
		payTo := []giga.PayTo{
			{Amount: dc("2"), PayTo: to_1, DeductFeePercent: dc("80")},
			{Amount: dc("1"), PayTo: to_2, DeductFeePercent: dc("20")},
		}
		source := giga.NewArrayUTXOSource(testUTXOs)
		txn, err := giga.CreateTxn(payTo, giga.ZeroCoins, acc, source, lib)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if txn.TxnHex == "" {
			t.Fatalf("missing tx hex")
		}
		if !txn.TotalIn.Equals(dc("3").Add(txn.ChangeAmount)) {
			t.Fatalf("wrong total inputs: %v", txn.TotalIn)
		}
		if !txn.FeeAmount.IsPositive() {
			t.Fatalf("zero or negative fee: %v", txn.FeeAmount)
		}
		if !txn.TotalOut.Equals(decimal.RequireFromString("3").Sub(txn.FeeAmount)) {
			t.Fatalf("wrong total outputs: %v", txn.TotalOut)
		}
	})
}

func dc(val string) decimal.Decimal {
	return decimal.RequireFromString(val)
}

func newTestRig(t *testing.T) giga.L1 {
	config := giga.TestConfig()
	mock, err := dogecoin.NewL1Mock(config)
	if err != nil {
		t.Fatalf("Cannot init L1 mock: %v", err)
	}
	lib, err := dogecoin.NewL1Libdogecoin(config, mock)
	if err != nil {
		t.Fatalf("Cannot init libdogecoin: %v", err)
	}
	return lib
}

func makeAccount(t *testing.T, foreignID string, lib giga.L1) giga.Account {
	addr, priv, err := lib.MakeAddress(true) // isTestNet
	if err != nil {
		t.Fatalf("makeAccount: cannot create address: %v", err)
	}
	return giga.Account{
		Address:   addr,
		ForeignID: foreignID,
		Privkey:   priv,
	}
}

func makeUTXO(t *testing.T, vout int, val string, acc *giga.Account, lib giga.L1) giga.UTXO {
	payTo, keyIndex, err := acc.NextPayToAddress(lib)
	if err != nil {
		t.Fatalf("NextPayToAddress: %v", err)
	}
	return giga.UTXO{
		TxID:          "3f8e64a8453377def77868188811c2c7ed25fb31a16957e0001e28774d6d0208",
		VOut:          vout,
		Value:         dc(val),
		ScriptHex:     p2pkhScriptHex(t, payTo),
		ScriptType:    giga.ScriptTypeP2PKH,
		ScriptAddress: payTo,
		AccountID:     acc.Address,
		KeyIndex:      keyIndex,
		IsInternal:    false,
		BlockHeight:   100,
	}
}

func p2pkhScriptHex(t *testing.T, addr giga.Address) string {
	payload, err := doge.Base58DecodeCheck(string(addr))
	if err != nil {
		t.Fatalf("Base58DecodeCheck: %v", err)
	}
	hash := doge.HexEncode(payload[1:]) // skip "version" byte.
	if len(hash) != 0x14*2 {
		t.Fatalf("wrong hash len: %v (need 20 bytes)", len(hash))
	}
	return "76a914" + hash + "88ac"
}
