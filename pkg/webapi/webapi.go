package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/julienschmidt/httprouter"
	"github.com/tjstebbing/conductor"
)

// WebAPI implements tjstebbing/conductor.Service
type WebAPI struct {
	srv  *http.Server
	bind string
	port string
	api  giga.API
}

// interface guard ensures WebAPI implements conductor.Service
var _ conductor.Service = WebAPI{}

func NewWebAPI(config giga.Config, l1 giga.L1, store giga.Store) (WebAPI, error) {
	return WebAPI{bind: config.WebAPI.Bind, port: config.WebAPI.Port, api: giga.NewAPI(store, l1)}, nil
}

func (t WebAPI) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		mux := httprouter.New()

		// Internal APIs

		// POST { account } /account/:foreignID -> { account } upsert account
		mux.POST("/account/:foreignID", t.upsertAccount)

		// GET /account/:foreignID -> { account } return an account
		mux.GET("/account/:foreignID", t.getAccount)

		// POST {invoice} /account/:foreignID/invoice/ -> { invoice } create new invoice
		mux.POST("/account/:foreignID/invoice/", t.createInvoice)

		// GET /account/:foreignID/invoice/:invoiceID -> { invoice } get an invoice
		mux.GET("/account/:foreignID/invoice/:invoiceID", t.getAccountInvoice)

		// GET /invoice/:invoiceID -> { invoice } get an invoice (sans account ID)
		mux.GET("/invoice/:invoiceID", t.getInvoice)

		// GET /account/:foreignID/invoices ? args -> [ {...}, ..] get all / filtered invoices
		mux.GET("/account/:foreignID/invoices", t.listInvoices)

		// POST /invoice/:invoiceID/payfrom/:foreignID -> { status } pay invoice from internal account
		mux.POST("/invoice/:invoiceID/payfrom/:foreignID", t.payInvoiceFromInternal)

		// POST /decode-txn -> test decoding
		mux.POST("/decode-txn", t.decodeTxn)

		// POST { amount } /invoice/:invoiceID/refundtoaddr/:address -> { status } refund all or part of a paid invoice to address

		// POST { amount } /invoice/:invoiceID/refundtoacc/:foreignID -> { status } refund all or part of a paid invoice to account

		// External APIs

		// GET /invoice/:invoiceID/connect -> { dogeConnect json } get the dogeConnect JSON for an invoice

		// GET /invoice/:invoiceID/status -> { status } get status of an invoice

		// GET /invoice/:invoiceID/poll -> { status } long-poll invoice waiting for status change

		// GET /invoice/:invoiceID/splash -> html page that tries to launch dogeconnect:// with QRcode fallback

		// POST { dogeConnect payment } /invoice/:invoiceID/pay -> { status } pay an invoice with a dogeConnect response

		t.srv = &http.Server{Addr: t.bind + ":" + t.port, Handler: mux}
		fmt.Printf("listening on %s:%s", t.bind, t.port)
		go func() {
			if err := t.srv.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("HTTP server ListenAndServe: %v", err)
			}
		}()
		started <- true
		ctx := <-stop
		t.srv.Shutdown(ctx)
		stopped <- true
	}()
	return nil
}

// createInvoice returns the ID of the created Invoice (which is the one-time address for this transaction) for the foreignID in the URL and the InvoiceCreateRequest in the body
func (t WebAPI) createInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing invoice ID in URL")
		return
	}
	var o giga.InvoiceCreateRequest
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	if o.Vendor == "" {
		sendBadRequest(w, "missing 'vendor' in JSON body")
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
	sendResponse(w, invoice)
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
	acc, err := t.api.GetAccount(id)
	if err != nil {
		sendError(w, "GetAccount", err)
		return
	}
	invoice, err := t.api.GetInvoice(giga.Address(id)) // TODO: need a "not found" error-code
	if err != nil {
		sendError(w, "GetInvoice", err)
		return
	}
	if invoice.Account != acc.Address {
		sendErrorResponse(w, 404, giga.NotFound, "no such invoice in this account")
		return
	}
	sendResponse(w, invoice)
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
	sendResponse(w, invoice)
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
	sendResponse(w, invoices)
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
	txn, err := t.api.PayInvoiceFromAccount(giga.Address(invoice_id), foreign_id)
	if err != nil {
		sendError(w, "PayInvoiceFromAccount", err)
		return
	}
	sendResponse(w, txn)
}

// upsertAccount returns the address of the new account with the foreignID in the URL
func (t WebAPI) upsertAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the foreignID is a 3rd-party ID for the account
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		sendBadRequest(w, "missing account ID in URL")
		return
	}
	acc, err := t.api.CreateAccount(foreignID, true)
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

type DecodeTxnRequest struct {
	Hex string `json:"hex"`
}

func (t WebAPI) decodeTxn(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var o DecodeTxnRequest
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	rawTxn, err := t.api.L1.DecodeTransaction(o.Hex)
	if err != nil {
		sendBadRequest(w, fmt.Sprintf("error decoding transaction: %v", err))
		return
	}
	sendResponse(w, rawTxn)
}
