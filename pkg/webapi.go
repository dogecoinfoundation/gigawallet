package giga

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/tjstebbing/conductor"
)

// WebAPI implements tjstebbing/conductor.Service
type WebAPI struct {
	srv  *http.Server
	port string
	api  API
}

// interface guard ensures WebAPI implements conductor.Service
var _ conductor.Service = WebAPI{}

func NewWebAPI(config Config, l1 L1, store Store) (WebAPI, error) {
	return WebAPI{port: config.WebAPI.Port, api: NewAPI(store, l1)}, nil
}

func (t WebAPI) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		mux := httprouter.New()

		// POST { account } /account/:foreignID -> { account } upsert account
		mux.POST("/account/:foreignID", t.upsertAccount)

		mux.POST("/invoice/:foreignID", t.createInvoice)
		mux.GET("/invoice/:invoiceID", t.getInvoice)
		mux.GET("/account/:foreignID", t.getAccount)
		mux.GET("/accountbyaddr/:address", t.getAccountByAddress) // TODO: figure out some way to to merge this and the above

		t.srv = &http.Server{Addr: ":" + t.port, Handler: mux}
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
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		t.sendError(w, 400, "bad-request", "bad request: missing invoice ID")
		return
	}
	var o InvoiceCreateRequest
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		t.sendError(w, 400, "bad-request", fmt.Sprintf("bad request body (expecting JSON): %v", err))
		return
	}
	i, err := t.api.CreateInvoice(o, foreignID)
	if err != nil {
		t.sendError(w, 500, "internal", fmt.Sprintf("error in CreateInvoice: %v", err))
		return
	}
	t.sendResponse(w, i.ID) // TODO: JSON object
}

// getInvoice is responsible for returning the current status of an invoice with the invoiceID in the URL
func (t WebAPI) getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the invoiceID is the address of the invoice
	id := p.ByName("invoiceID")
	if id == "" {
		t.sendError(w, 400, "bad-request", "bad request: missing invoice ID")
		return
	}
	invoice, err := t.api.GetInvoice(Address(id))
	if err != nil {
		t.sendError(w, 500, "internal", fmt.Sprintf("error in GetInvoice: %v", err))
		return
	}
	t.sendResponse(w, invoice)
}

// upsertAccount returns the address of the new account with the foreignID in the URL
func (t WebAPI) upsertAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		t.sendError(w, 400, "bad-request", "bad request: missing foreign ID")
		return
	}
	acc, err := t.api.CreateAccount(foreignID, true)
	if err != nil {
		t.sendError(w, 500, "internal", fmt.Sprintf("error in CreateAccount: %v", err))
		return
	}
	t.sendResponse(w, acc)
}

// getAccount returns the public info of the account with the foreignID in the URL
func (t WebAPI) getAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the id is the address of the invoice
	id := p.ByName("foreignID")
	if id == "" {
		t.sendError(w, 400, "bad-request", "bad request: missing foreign ID")
		return
	}
	acc, err := t.api.GetAccount(id)
	if err != nil {
		t.sendError(w, 500, "internal", fmt.Sprintf("error in GetAccount: %v", err))
		return
	}
	t.sendResponse(w, acc)
}

// getAccountByAddress returns the public info of the account with the address in the URL
func (t WebAPI) getAccountByAddress(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// address of the account
	id := p.ByName("address")
	if id == "" {
		t.sendError(w, 400, "bad-request", "bad request: missing account address")
		return
	}
	acc, err := t.api.GetAccountByAddress(Address(id))
	if err != nil {
		t.sendError(w, 500, "internal", fmt.Sprintf("error in GetAccountByAddress: %v", err))
		return
	}
	t.sendResponse(w, acc)
}

type WebError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WebErrorResponse struct {
	Error WebError `json:"error"`
}

func (t WebAPI) sendResponse(w http.ResponseWriter, payload any) {
	// note: w.Header after this, so we can call t.sendError
	b, err := json.Marshal(payload)
	if err != nil {
		t.sendError(w, 500, "marshal", fmt.Sprintf("json.marshal error: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func (t WebAPI) sendError(w http.ResponseWriter, statusCode int, code string, message string) {
	// note: w.Header here, because we always write a response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	payload := &WebErrorResponse{
		Error: WebError{Code: code, Message: message},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(w, "[!] error sending %d (%s) error: %v", statusCode, code, err)
		w.Write([]byte("{error:{code:\"marshal\"}}"))
		return
	}
	w.Write(b)
}
