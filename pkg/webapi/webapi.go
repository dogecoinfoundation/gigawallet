package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
	"github.com/julienschmidt/httprouter"
)

// WebAPI implements conductor.Service
type WebAPI struct {
	api    giga.API
	config giga.Config
}

// interface guard ensures WebAPI implements conductor.Service
var _ conductor.Service = WebAPI{}

func NewWebAPI(config giga.Config, api giga.API) (WebAPI, error) {
	return WebAPI{api: api, config: config}, nil
}

func (t WebAPI) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		adminMux, pubMux := t.createRouters()

		// Start the admin server
		adminServer := &http.Server{Addr: t.config.WebAPI.AdminBind + ":" + t.config.WebAPI.AdminPort, Handler: adminMux}
		fmt.Printf("\nAdmin API listening on %s:%s", t.config.WebAPI.AdminBind, t.config.WebAPI.AdminPort)
		go func() {
			if err := adminServer.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("HTTP server admin ListenAndServe: %v", err)
			}
		}()

		// Start the public server
		pubServer := &http.Server{Addr: t.config.WebAPI.PubBind + ":" + t.config.WebAPI.PubPort, Handler: pubMux}
		fmt.Printf("\nPublic API listening on %s:%s", t.config.WebAPI.PubBind, t.config.WebAPI.PubPort)
		go func() {
			if err := pubServer.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("HTTP server public ListenAndServe: %v", err)
			}
		}()

		started <- true
		ctx := <-stop
		adminServer.Shutdown(ctx)
		pubServer.Shutdown(ctx)
		stopped <- true
	}()
	return nil
}

func (t WebAPI) authMiddleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if t.config.WebAPI.AdminBearerToken == "" {
			// Skip auth if the user hasn't configured a token.
			handler(w, r, ps)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendErrorResponse(w, http.StatusUnauthorized, giga.Unauthorized, "Authorization header required")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			sendErrorResponse(w, http.StatusUnauthorized, giga.Unauthorized, "Invalid authorization format, expected 'Bearer TOKEN'")
			return
		}

		if parts[1] != t.config.WebAPI.AdminBearerToken {
			sendErrorResponse(w, http.StatusUnauthorized, giga.Unauthorized, "Invalid token")
			return
		}

		handler(w, r, ps)
	}
}

func (t WebAPI) createRouters() (adminMux *httprouter.Router, pubMux *httprouter.Router) {
	adminMux = httprouter.New() // Admin APIs
	pubMux = httprouter.New()   // Public APIs

	// Admin APIs

	adminMux.POST("/admin/setsyncheight/:blockheight", t.authMiddleware(t.setSyncHeight))

	// POST { account } /account/:foreignID -> { account } upsert account
	adminMux.POST("/account/:foreignID", t.authMiddleware(t.upsertAccount))

	// GET /account/:foreignID -> { account } return an account
	adminMux.GET("/account/:foreignID", t.authMiddleware(t.getAccount))

	// GET /account:foreignID/Balance -> { AccountBalance    Get the account balance
	adminMux.GET("/account/:foreignID/balance", t.authMiddleware(t.getAccountBalance))

	// POST {invoice} /account/:foreignID/invoice -> { invoice } create new invoice
	adminMux.POST("/account/:foreignID/invoice", t.authMiddleware(t.createInvoice))
	adminMux.POST("/account/:foreignID/invoice/", t.authMiddleware(t.createInvoice)) // deprecated: prior bug

	// GET /account/:foreignID/invoices ? args -> [ {...}, ..] get all / filtered invoices
	adminMux.GET("/account/:foreignID/invoices", t.authMiddleware(t.listInvoices))

	// GET /account/:foreignID/invoice/:invoiceID -> { invoice } get an invoice
	adminMux.GET("/account/:foreignID/invoice/:invoiceID", t.authMiddleware(t.getAccountInvoice))

	// POST /account/:foreignID/pay { "amount": "1.0", "to": "DPeTgZm7LabnmFTJkAPfADkwiKreEMmzio" } -> { status }
	adminMux.POST("/account/:foreignID/pay", t.authMiddleware(t.payToAddress))

	// POST /account/:foreignID/paytx { "pay": [{ "amount":"1.0", "to": "DPeTgZm7LabnmFTJkAPfADkwiKreEMmzio" }] } -> { tx }
	adminMux.POST("/account/:foreignID/paytx", t.authMiddleware(t.payTransaction))

	// POST /invoice/:invoiceID/payfrom/:foreignID -> { status } pay invoice from internal account
	adminMux.POST("/invoice/:invoiceID/payfrom/:foreignID", t.authMiddleware(t.payInvoiceFromInternal))

	// POST { amount } /invoice/:invoiceID/refundtoaddr/:address -> { status } refund all or part of a paid invoice to address

	// POST { amount } /invoice/:invoiceID/refundtoacc/:foreignID -> { status } refund all or part of a paid invoice to account

	// External APIs

	// GET /invoice/:invoiceID -> { invoice } get an invoice (sans account ID)
	pubMux.GET("/invoice/:invoiceID", t.getInvoice)

	pubMux.GET("/invoice/:invoiceID/qr.png", t.getInvoiceQR)

	// DEPRECATED, use /dc/:invoiceID (this URL needs to be short)
	pubMux.GET("/invoice/:invoiceID/connect", t.dcGetEnvelope)

	// GET /invoice/:invoiceID/connect -> { dogeConnect json } get the dogeConnect JSON for an invoice
	// GET /invoice/:invoiceID/status -> { status } get status of an invoice
	// GET /invoice/:invoiceID/poll -> { status } long-poll invoice waiting for status change
	// GET /invoice/:invoiceID/splash -> html page that tries to launch dogeconnect:// with QRcode fallback
	// POST { dogeConnect payment } /invoice/:invoiceID/pay -> { status } pay an invoice with a dogeConnect response

	// External DogeConnect APIs

	pubMux.GET("/dc/:invoiceID", t.dcGetEnvelope)

	pubMux.POST("/dc/:invoiceID/pay", t.dcPayInvoice)

	pubMux.POST("/dc/:invoiceID/status", t.dcPayStatus)

	return
}

// SetSyncHeight resets the sync height for GigaWallet, which will cause
// chain-follower to start re-scanning the blockchain from that point.
// This can be used when starting a NEW GigaWallet instance to avoid
// scanning the entire blockchain, if you have no imported wallets with
// old transactions.
//
// WARNING:  Using this on an active/production GigaWallet will PAUSE
// the discovery of any new transactions until the re-scan has completed,
// meaning that any users waiting for Invoice Paid confirmations will
// be on hold until everything is reindexed. USE WITH CAUTION.
func (t WebAPI) setSyncHeight(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	n, err := strconv.ParseInt(p.ByName("blockheight"), 10, 64)
	if err != nil {
		sendBadRequest(w, "blockheight invalid, must convert to int64")
		return
	}

	err = t.api.SetSyncHeight(n)
	if err != nil {
		sendError(w, "SetSyncHeight failed", err)
		return
	}
	sendResponse(w, "Set sync height")
}

// createInvoice returns the ID of the created Invoice (which is the one-time address for this transaction) for the foreignID in the URL and the InvoiceCreateRequest in the body
func (t WebAPI) createInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing invoice ID in URL")
		return
	}
	o := giga.InvoiceCreateRequest{
		Confirmations: -1,
	}
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	if len(o.Items) < 1 {
		sendBadRequest(w, "missing 'items' in JSON body")
		return
	}
	invoice, err := t.api.CreateInvoice(o, foreignID)
	if err != nil {
		sendError(w, "CreateInvoice", err)
		return
	}
	sendResponse(w, invoice.ToPublic())
}

// getAccountInvoice is responsible for returning the current status of an invoice with the invoiceID in the URL
func (t WebAPI) getAccountInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	// the invoiceID is the address of the invoice
	id := p.ByName("invoiceID")
	if id == "" {
		sendBadRequest(w, "missing invoice ID")
		return
	}
	acc, err := t.api.GetAccount(foreignID)
	if err != nil {
		sendError(w, "GetAccount", err)
		return
	}
	invoice, err := t.api.GetInvoice(giga.Address(id))
	if err != nil {
		sendErrorResponse(w, 404, giga.NotFound, "no such invoice in this account")
		return
	}
	if invoice.Account != acc.Address {
		sendErrorResponse(w, 404, giga.NotFound, "no such invoice in this account")
		return
	}
	sendResponse(w, invoice)
}

func (t WebAPI) getInvoiceQR(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	// the invoiceID is the address of the invoice
	id := p.ByName("invoiceID")
	if id == "" {
		sendBadRequest(w, "missing invoice ID")
		return
	}
	invoice, err := t.api.GetInvoice(giga.Address(id))
	if err != nil {
		sendErrorResponse(w, 404, giga.NotFound, "no such invoice")
		return
	}

	uri, err := t.api.GetInvoiceConnectURL(invoice, t.config.WebAPI.PubAPIRootURL)
	if err != nil {
		sendErrorResponse(w, 404, giga.UnknownError, err.Error())
		return
	}

	qs := r.URL.Query()
	fg := qs.Get("fg")
	bg := qs.Get("bg")
	qr, _ := GenerateQRCodePNG(uri, 512, fg, bg)

	//  Maxage 900 (15 minutes) is because this image should not
	//  change at all for a given invoice and we expect most invoices
	// to be complete in far less time than 15 min.. but 15 min allows
	// us room to upgrade the format between releases if needed..
	w.Header().Set("Cache-Control", "max-age:=900, immutable")
	w.Header().Set("Content-Type", "image/png")
	w.Write(qr)
}

// getInvoice is responsible for returning the current status of an invoice with the invoiceID in the URL
func (t WebAPI) getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the invoiceID is the address of the invoice
	id := p.ByName("invoiceID")
	if id == "" {
		sendBadRequest(w, "missing invoice ID")
		return
	}
	invoice, err := t.api.GetInvoice(giga.Address(id))
	if err != nil {
		sendError(w, "GetInvoice", err)
		return
	}
	sendResponse(w, invoice.ToPublic())
}

// listInvoices is responsible for returning a list of invoices and their status for an account
func (t WebAPI) listInvoices(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	// optional pagination: cursor comes from the previous response (or zero)
	icursor := 0
	ilimit := 10
	qs := r.URL.Query()
	cursor := qs.Get("cursor")
	var err error
	if cursor != "" {
		icursor, err = strconv.Atoi(cursor)
		if err != nil || icursor < 0 {
			sendBadRequest(w, "invalid cursor in URL")
			return
		}
	}
	limit := qs.Get("limit")
	if limit != "" {
		ilimit, err = strconv.Atoi(limit)
		if err != nil || ilimit < 1 {
			sendBadRequest(w, "invalid limit in URL")
			return
		}
		if ilimit > 100 {
			sendBadRequest(w, "invalid limit in URL (cannot be greater than 100)")
			return
		}
	}
	invoices, err := t.api.ListInvoices(foreignID, icursor, ilimit)
	if err != nil {
		sendError(w, "ListInvoices", err)
		return
	}
	items := invoices.Items
	for i, inv := range items {
		inv.AddPublic()
		items[i] = inv // mutate slice
	}
	sendResponse(w, invoices)
}

type ListInvoicesResponse struct {
	Items  []giga.Invoice `json:"items"`
	Cursor int            `json:"cursor"`
}

type PayToAddressRequest struct {
	Amount      giga.CoinAmount `json:"amount"`
	PayTo       giga.Address    `json:"to"`
	ExplicitFee giga.CoinAmount `json:"explicit_fee"` // optional fee override (missing or zero: calculate the fee)
	MaxFee      giga.CoinAmount `json:"max_fee"`      // optional maximum fee (missing or zero: maximum is 1 DOGE)
	Pay         []giga.PayTo    `json:"pay"`          // either Pay, or Amount and PayTo.
}

type PayToAddressResponse = giga.SendFundsResult

// Pays funds from an account managed by gigawallet to any Dogecoin Address.
// POST /account/:foreignID/pay { "amount": "1.0", "to": "DPeTgZm7LabnmFTJkAPfADkwiKreEMmzio" } -> { status }
// or { "explicit_fee": "0.2", "pay": [ "amount": "1.0", "to": "DPeT…", "deduct_fee_percent": "100" ] }
func (t WebAPI) payToAddress(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	var o PayToAddressRequest
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	if len(o.Pay) == 0 {
		// treat 'PayTo' request as an array of one item.
		o.Pay = append(o.Pay, giga.PayTo{Amount: o.Amount, PayTo: o.PayTo})
	}
	res, err := t.api.SendFundsToAddress(foreignID, o.Pay, o.ExplicitFee, o.MaxFee, true)
	if err != nil {
		sendError(w, "SendFundsToAddress", err)
		return
	}
	sendResponse(w, res)
}

// Create and sign a transaction to pay out funds from the account.
// POST /account/:foreignID/paytx { "pay":[{ "amount": "1.0", "to": "DPeT…" }] } -> { tx:"…hex" }
// or { "explicit_fee": "0.2", "pay": [ "amount": "1.0", "to": "DPeT…", "deduct_fee_percent": "100" ] }
func (t WebAPI) payTransaction(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	var o PayToAddressRequest
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	if len(o.Pay) == 0 {
		// treat 'PayTo' request as an array of one item.
		o.Pay = append(o.Pay, giga.PayTo{Amount: o.Amount, PayTo: o.PayTo})
	}
	res, err := t.api.SendFundsToAddress(foreignID, o.Pay, o.ExplicitFee, o.MaxFee, false)
	if err != nil {
		sendError(w, "PayTransaction", err)
		return
	}
	sendResponse(w, res)
}

// pays an invoice from another account managed by gigawallet
// POST /invoice/:invoiceID/payfrom/:foreignID -> { status }
func (t WebAPI) payInvoiceFromInternal(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	invoice_id := p.ByName("invoiceID")
	if invoice_id == "" {
		sendBadRequest(w, "missing invoice ID in URL")
		return
	}
	foreign_id := p.ByName("foreignID")
	if foreign_id == "" {
		sendBadRequest(w, "missing foreign ID in URL")
		return
	}
	res, err := t.api.PayInvoiceFromAccount(giga.Address(invoice_id), foreign_id)
	if err != nil {
		sendError(w, "PayInvoiceFromAccount", err)
		return
	}
	sendResponse(w, res)
}

// upsertAccount returns the address of the new account with the foreignID in the URL
func (t WebAPI) upsertAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	o := giga.AccountCreateRequest{}
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	acc, err := t.api.CreateAccount(o, foreignID, true)
	if err != nil {
		sendError(w, "CreateAccount", err)
		return
	}
	sendResponse(w, acc)
}

// getAccount returns the public info of the account with the foreignID in the URL
func (t WebAPI) getAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	id := p.ByName("foreignID")
	if id == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	acc, err := t.api.GetAccount(id)
	if err != nil {
		sendError(w, "GetAccount", err)
		return
	}
	sendResponse(w, acc)
}

func (t WebAPI) getAccountBalance(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	id := p.ByName("foreignID")
	if id == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	bal, err := t.api.CalculateBalance(id)
	if err != nil {
		sendError(w, "CalculateBalance", err)
		return
	}
	sendResponse(w, bal)
}

// DogeConnect APIs

func (t WebAPI) dcGetEnvelope(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the invoiceID is the address of the invoice
	id := p.ByName("invoiceID")
	if id == "" {
		sendBadRequest(w, "missing invoice ID")
		return
	}
	invoice, err := t.api.GetInvoice(giga.Address(id))
	if err != nil {
		sendErrorResponse(w, 404, giga.NotFound, "no such invoice")
		return
	}

	envelope, err := t.api.GetInvoiceConnectEnvelope(invoice, t.config.WebAPI.PubAPIRootURL)
	if err != nil {
		sendErrorResponse(w, 500, giga.UnknownError, err.Error())
		return
	}

	// Maxage 900 (15 minutes) is because this envelope should not
	// change at all for a given invoice and we expect most invoices
	// to be complete in far less time than 15 min.. but 15 min allows
	// us room to upgrade the format between releases if needed..
	w.Header().Set("Cache-Control", "max-age:=900, immutable")
	sendResponse(w, envelope)
}

func (t WebAPI) dcPayInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
}

func (t WebAPI) dcPayStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
}
