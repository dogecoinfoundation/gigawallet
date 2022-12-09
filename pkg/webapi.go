package giga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/tjstebbing/conductor"
)

// WebAPI implements tjstebbing/conductor.Service
type WebAPI struct {
	srv  *http.Server
	bind string
	port string
	api  API
}

// interface guard ensures WebAPI implements conductor.Service
var _ conductor.Service = WebAPI{}

func NewWebAPI(config Config, l1 L1, store Store) (WebAPI, error) {
	return WebAPI{bind: config.WebAPI.Bind, port: config.WebAPI.Port, api: NewAPI(store, l1)}, nil
}

func (t WebAPI) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		mux := httprouter.New()

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
		mux.GET("/account/:foreignID/invoices?limit=:limit", t.listInvoices)

		// POST /invoice/:invoiceID/payfrom/:foreignID -> { status } pay invoice from internal account

		// POST { ammount } /invoice/:invoiceID/refundtoaddr/:address -> { status } refund all or part of a paid invoice to address

		// POST { ammount } /invoice/:invoiceID/refundtoacc/:foreignID -> { status } refund all or part of a paid invoice to account

		t.srv = &http.Server{Addr: t.bind + ":" + t.port, Handler: mux}
		go func() {
			if err := t.srv.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("HTTP server ListenAndServe: %v", err)
			}
		}()
		started <- true
		select {
		case ctx := <-stop:
			// do some shutdown stuff then signal we're done
			t.srv.Shutdown(ctx)
			stopped <- true
		}
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
	var o InvoiceCreateRequest
	jerr := json.NewDecoder(r.Body).Decode(&o)
	if jerr != nil {
		sendBadRequest(w, fmt.Sprintf("bad request body (expecting JSON): %v", jerr))
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
	invoice, err := t.api.GetInvoice(Address(id)) // TODO: need a "not found" error-code
	if err != nil {
		sendError(w, "GetInvoice", err)
		return
	}
	if invoice.Account != acc.Address {
		sendErrorResponse(w, 404, NotFound, "no such invoice in this account")
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
	invoice, err := t.api.GetInvoice(Address(id))
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
	var perr error
	if cursor != "" {
		icursor, perr = strconv.Atoi(cursor)
		if perr != nil || icursor < 0 {
			sendBadRequest(w, "invalid cursor in URL")
			return
		}
	}
	limit := qs.Get("limit")
	if limit != "" {
		ilimit, perr = strconv.Atoi(limit)
		if perr != nil || ilimit < 1 {
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

// Helpers

func sendResponse(w http.ResponseWriter, payload any) {
	// note: w.Header after this, so we can call sendError
	b, err := json.Marshal(payload)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "marshal", fmt.Sprintf("in json.Marshal: %s", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // do not cache (Browsers cache GET forever by default)
	w.Write(b)
}

func sendBadRequest(w http.ResponseWriter, message string) {
	sendErrorResponse(w, http.StatusBadRequest, BadRequest, message)
}

func sendError(w http.ResponseWriter, where string, err error) {
	var info *ErrorInfo
	if errors.As(err, &info) {
		status := HttpStatusForError(info.Code)
		message := fmt.Sprintf("in %s: %s", where, info.Message)
		sendErrorResponse(w, status, info.Code, message)
	} else {
		message := fmt.Sprintf("in %s: %s", where, err.Error())
		sendErrorResponse(w, http.StatusInternalServerError, UnknownError, message)
	}
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, code ErrorCode, message string) {
	log.Printf("[!] %s: %s\n", code, message)
	// would prefer to use json.Marshal, but this avoids the need
	// to handle encoding errors arising from json.Marshal itself!
	payload := fmt.Sprintf("{\"error\":{\"code\":%q,\"debug\":%q}}", code, message)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // do not cache (Browsers cache GET forever by default)
	w.WriteHeader(statusCode)
	w.Write([]byte(payload))
}
