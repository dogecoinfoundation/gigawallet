package giga

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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
		mux.POST("/invoice/:foreignID", t.createInvoice)
		mux.GET("/invoice/:invoiceID", t.protectInvoiceRoute(t.getInvoice))
		mux.POST("/account/:foreignID", t.createAccount)
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
		http.Error(w, "error: missing foreign ID", http.StatusBadRequest)
		return
	}
	var o InvoiceCreateRequest
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad request body: %v", err), http.StatusBadRequest)
		return
	}
	i, err := t.api.CreateInvoice(o, foreignID)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %v", err), http.StatusInternalServerError)
		return
	}
	b, err := json.Marshal(i.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", string(b))
}

// getInvoice is responsible for returning the current status of an invoice with the invoiceID in the URL
func (t WebAPI) getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the invoiceID is the address of the invoice
	id := p.ByName("invoiceID")
	if id == "" {
		fmt.Fprintf(w, "error: missing invoice ID")
		return
	}
	invoice, err := t.api.GetInvoice(Address(id))
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	b, err := json.Marshal(invoice)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", string(b))
}

// createAccount returns the address of the new account with the foreignID in the URL
func (t WebAPI) createAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		fmt.Fprintf(w, "error: missing foreign ID")
		return
	}
	addr, err := t.api.CreateAccount(foreignID)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	fmt.Fprintf(w, "%s", string(addr))
}

// getAccount returns the public info of the account with the foreignID in the URL
func (t WebAPI) getAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the id is the address of the invoice
	id := p.ByName("foreignID")
	if id == "" {
		fmt.Fprintf(w, "error: missing foreign ID")
		return
	}
	acc, err := t.api.GetAccount(id)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	b, err := json.Marshal(acc)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", string(b))
}

// getAccountByAddress returns the public info of the account with the address in the URL
func (t WebAPI) getAccountByAddress(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// address of the account
	id := p.ByName("address")
	if id == "" {
		fmt.Fprintf(w, "error: missing account address")
		return
	}
	acc, err := t.api.GetAccountByAddress(Address(id))
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	b, err := json.Marshal(acc)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", string(b))
}

/* Wraps a route handler and ensures Authorization header matches invoice token */
func (t WebAPI) protectInvoiceRoute(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		id := p.ByName("invoiceID")
		if id == "" {
			fmt.Fprintf(w, "error: missing invoice ID")
			return
		}
		invoice, err := t.api.GetInvoice(Address(id))
		if err != nil {
			fmt.Fprintf(w, "error: invoice not found")
			return
		}

		//Check authorization header matches
		bits := strings.Split(strings.ToUpper(r.Header.Get("Authorization")), "TOKEN ")
		if len(bits) == 2 && invoice.AccessToken == bits[1] {
			h(w, r, p)
			return
		}

		fmt.Fprintf(w, "error: invalid access token for invoice")
	}
}
