package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	dbstore "github.com/dogecoinfoundation/gigawallet/pkg/store"
	"github.com/julienschmidt/httprouter"
	"github.com/shopspring/decimal"
)

func TestWebAPI(t *testing.T) {
	admin, _, store, l1 := newTestRig(t) // _ = pub

	// Create Account "Pepper"
	var pepper giga.AccountPublic
	request(t, admin, "/account/Pepper", `{}`, &pepper)
	if pepper.ForeignID != "Pepper" {
		t.Fatalf("Create Account did not round-trip foreignID: %s", pepper.ForeignID)
	}
	if !doge.ValidateP2PKH(string(pepper.Address), &doge.DogeTestNetChain) {
		t.Fatalf("Create Account generated an invalid account ID: %v", pepper.Address)
	}

	// Create Account "FeeFee" with config
	var feefee giga.AccountPublic
	request(t, admin, "/account/FeeFee", `{"payout_address":"xyz","payout_threshold":"10","payout_frequency":"1"}`, &feefee)

	// Get Account "Pepper"
	var pepper2 giga.AccountPublic
	request(t, admin, "/account/Pepper", "", &pepper2)
	if pepper2.Address != pepper.Address {
		t.Fatalf("Account did not round-trip Account ID (Address): %v vs %v", pepper2.Address, pepper.Address)
	}

	// Get Balance for "Pepper"
	var balance giga.AccountBalance
	request(t, admin, "/account/Pepper/balance", "", &balance)
	if !balance.CurrentBalance.IsZero() {
		t.Fatalf("Account has unexpected balance: %v", balance.IncomingBalance)
	}

	// Create an Invoice for 10 doge
	var inv1 giga.PublicInvoice
	request(t, admin, "/account/Pepper/invoice/", `{"items":[{"type":"item","name":"Pants","sku":"P-001","description":"Nice pants","value":"10","quantity":1}],"confirmations":6}`, &inv1)
	if !doge.ValidateP2PKH(string(inv1.ID), &doge.DogeTestNetChain) {
		t.Fatalf("Create Invoice generated an invalid Address: %v", inv1.ID)
	}

	// Get and compare the invoice
	var inv2 giga.PublicInvoice
	request(t, admin, "/account/Pepper/invoice/"+string(inv1.ID), "", &inv2)
	if !(inv1.ID == inv2.ID && inv1.Total.Equals(inv2.Total) && len(inv1.Items) == len(inv2.Items)) {
		t.Fatalf("Get Invoice did not return matching data: %v %v %v %v %v %v", inv1.ID, inv2.ID, inv1.Total, inv2.Total, len(inv1.Items), len(inv2.Items))
	}

	// List invoices
	var inv_l ListInvoicesPublicResponse
	request(t, admin, "/account/Pepper/invoices?cursor=0&limit=10", "", &inv_l)
	if inv_l.Cursor != 0 {
		t.Fatalf("List Invoices: expected zero cursor (end of list) %v", inv_l.Cursor)
	}
	if len(inv_l.Items) != 1 {
		t.Fatalf("List Invoices: expected a single Invoice, %v", len(inv_l.Items))
	}
	inv3 := inv_l.Items[0]
	if !(inv1.ID == inv3.ID && inv1.Total.Equals(inv3.Total) && len(inv1.Items) == len(inv3.Items)) {
		t.Fatalf("List Invoices: did not return matching data: %v %v %v %v %v %v", inv1.ID, inv3.ID, inv1.Total, inv3.Total, len(inv1.Items), len(inv3.Items))
	}

	// Add funds to account for payment tests
	to_1, to_2 := addFundsToAccount(t, store, l1, "Pepper")

	// Pay to Address
	var payTo PayToAddressResponse
	request(t, admin, "/account/Pepper/pay", `{"amount":"2","to":"`+to_1+`"}`, &payTo)
	if payTo.TxId == "" {
		t.Fatalf("Pay To Address 1: missing txid")
	}
	if !payTo.Fee.Equals(decimal.RequireFromString("0.00226")) {
		t.Fatalf("Pay To Address 1: wrong fee: %v", payTo.Fee)
	}

	// Pay with explicit fee
	request(t, admin, "/account/Pepper/pay", `{"amount":"2","to":"`+to_1+`","explicit_fee":"1"}`, &payTo)
	if payTo.TxId == "" {
		t.Fatalf("Pay To Address 2: missing txid")
	}
	if !payTo.Total.Equals(decimal.RequireFromString("3")) {
		t.Fatalf("Pay To Address 2: wrong total: %v", payTo.Total)
	}
	if !payTo.Fee.Equals(decimal.RequireFromString("1")) {
		t.Fatalf("Pay To Address 2: wong fee: %v", payTo.Fee)
	}
	if !payTo.Paid.Equals(decimal.RequireFromString("2")) {
		t.Fatalf("Pay To Address 2: wrong paid: %v", payTo.Paid)
	}

	// Pay to Multiple Addresses with percentage split
	request(t, admin, "/account/Pepper/pay", `{"pay":[{"amount":"2","to":"`+to_1+`","deduct_fee_percent":"80"},{"amount":"1","to":"`+to_2+`","deduct_fee_percent":"20"}]}`, &payTo)
	if payTo.TxId == "" {
		t.Fatalf("Pay To Address 3: missing txid")
	}
	if !payTo.Total.Equals(decimal.RequireFromString("3")) { // TEST CURRENTLY FAILS (need to implement deduct_fee_percent)
		t.Fatalf("Pay To Address 3: wrong total: %v", payTo.Total)
	}
	if !payTo.Fee.Equals(decimal.RequireFromString("0.0026")) {
		t.Fatalf("Pay To Address 3: wrong fee: %v", payTo.Fee)
	}
	if !payTo.Paid.Equals(decimal.RequireFromString("2.99741")) {
		t.Fatalf("Pay To Address 3: wrong paid: %v", payTo.Paid)
	}
}

// Helpers.

func request(t *testing.T, adminMux *httprouter.Router, path string, body string, out any) *http.Response {
	var reader *strings.Reader
	method := "GET"
	if body != "" {
		method = "POST"
	}
	reader = strings.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	res := httptest.NewRecorder()
	adminMux.ServeHTTP(res, req)
	result := res.Result()
	if result.StatusCode != 200 {
		t.Fatalf("%s request failed: %v %v", path, result.StatusCode, res.Body)
	}
	err := json.NewDecoder(res.Body).Decode(out)
	if err != nil {
		t.Fatalf("%s bad json: %v", path, res.Body)
	}
	return result
}

func newTestRig(t *testing.T) (admin *httprouter.Router, pub *httprouter.Router, store giga.Store, L1 giga.L1) {
	config := giga.TestConfig()
	store, err := dbstore.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Cannot create in-memory database: %v", err)
	}
	mock, err := dogecoin.NewL1Mock(config)
	if err != nil {
		t.Fatalf("Cannot init L1 mock: %v", err)
	}
	l1, err := dogecoin.NewL1Libdogecoin(config, mock)
	if err != nil {
		t.Fatalf("Cannot init libdogecoin: %v", err)
	}
	bus := giga.NewMessageBus()
	mockFollower := giga.MockFollower{}
	api := giga.NewAPI(store, l1, bus, &mockFollower, config)
	web := WebAPI{api: api, config: config}
	adminMux, pubMux := web.createRouters()
	return adminMux, pubMux, store, l1
}

func addFundsToAccount(t *testing.T, store giga.Store, l1 giga.L1, foreignID string) (string, string) {
	tx, err := store.Begin()
	if err != nil {
		t.Fatalf("store.Begin: %v", err)
	}
	acc, err := tx.GetAccount(foreignID)
	if err != nil {
		t.Fatalf("tx.GetAccount: %v", err)
	}
	to_1, err := acc.NextChangeAddress(l1)
	if err != nil {
		t.Fatalf("NextChangeAddress: %v", err)
	}
	to_2, err := acc.NextChangeAddress(l1)
	if err != nil {
		t.Fatalf("NextChangeAddress: %v", err)
	}
	// Insert 10 x P2PKH UTXOs with Value=10 and AccountID set.
	for vout := 0; vout < 10; vout++ {
		payTo, keyIndex, err := acc.NextPayToAddress(l1)
		if err != nil {
			t.Fatalf("NextPayToAddress: %v", err)
		}
		err = tx.CreateUTXO(giga.UTXO{
			TxID:          "3f8e64a8453377def77868188811c2c7ed25fb31a16957e0001e28774d6d0208",
			VOut:          vout,
			Value:         decimal.NewFromInt(10),
			ScriptHex:     p2pkhScriptHex(t, payTo),
			ScriptType:    giga.ScriptTypeP2PKH,
			ScriptAddress: payTo,
			AccountID:     acc.Address,
			KeyIndex:      keyIndex,
			IsInternal:    false,
			BlockHeight:   100,
		})
		if err != nil {
			t.Fatalf("tx.CreateUTXO: %v", err)
		}
	}
	err = tx.UpdateAccount(acc)
	if err != nil {
		t.Fatalf("tx.UpdateAccount: %v", err)
	}
	// Currently Gigawallet won't spend UTXOs until they are confirmed.
	_, err = tx.ConfirmUTXOs(6, 120) // 100 + 6 <= 120
	if err != nil {
		t.Fatalf("tx.ConfirmUTXOs: %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("tx.Commit: %v", err)
	}
	return string(to_1), string(to_2)
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
