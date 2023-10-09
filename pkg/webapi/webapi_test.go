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
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
	"github.com/julienschmidt/httprouter"
)

func TestWebAPI(t *testing.T) {
	admin, _ := newTestRig(t) // _ = pub

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

	// Insert some funds into the account

	// Pay to Address
	var payTo PayToAddressResponse
	request(t, admin, "/account/Pepper/pay", `{"amount":"2","to":"xyzzy"}`, &payTo)
	if payTo.TxId == "" {
		t.Fatalf("Pay To Address: missing txid")
	}

	// Pay to Multiple Addresses with percentage split
	request(t, admin, "/account/Pepper/pay", `{"pay":[{"amount":"2","to":"xyzzy","deduct_fee_percent":"80"},{"amount":"1","to":"abraca","deduct_fee_percent":"20"}]`, &payTo)
	if payTo.TxId == "" {
		t.Fatalf("Pay To Address: missing txid")
	}

	// Pay with explicit fee
	request(t, admin, "/account/Pepper/pay", `{"amount":"2","to":"xyzzy","explicit_fee":"1"}`, &payTo)
	if payTo.TxId == "" {
		t.Fatalf("Pay To Address: missing txid")
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

func newTestRig(t *testing.T) (adminMux *httprouter.Router, pubMux *httprouter.Router) {
	config := giga.TestConfig()
	store, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Cannot create in-memory database: %v", err)
	}
	l1, _ := dogecoin.NewL1Libdogecoin(config, nil)
	bus := giga.NewMessageBus()
	var mockFollower giga.MockFollower
	api := giga.NewAPI(store, l1, bus, &mockFollower, config)

	web := WebAPI{api: api, config: config}
	adminMux, pubMux = web.createRouters()
	return
}
