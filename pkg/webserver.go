package giga

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
	"github.com/julienschmidt/httprouter"
)

// PaymentAPIService implements tjstebbing/conductor.Service
type PaymentAPIService struct {
	srv  *http.Server
	port string
	api  API
}

func NewPaymentAPIService(config Config) (PaymentAPIService, error) {
	l1, err := dogecoin.NewL1Libdogecoin(config)
	if err != nil {
		return PaymentAPIService{}, err
	}
	// TODO: this uses a mock store
	api := NewAPI(store.NewMock(), l1)
	return PaymentAPIService{port: config.PaymentService.Port, api: api}, nil
}

func (t PaymentAPIService) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		mux := httprouter.New()
		mux.POST("/invoice", t.createInvoice)
		mux.GET("/invoice/:id", t.getInvoice)
		mux.POST("/account/:foreignID", t.createAccount)
		mux.GET("/account/:foreignID", t.getAccount)
		mux.GET("/account/byaddress/:address", t.getAccountByAddress) // TODO: figure out some way to to merge this and the above

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

func (t PaymentAPIService) createInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var o Invoice
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}

	// o.ID right now is actually the foreignID, so convert it to the address
	account, err := t.api.GetAccount(string(o.ID))
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
		return
	}
	o.ID = account.Address

	err = t.api.StoreInvoice(o)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
	}
}

// getInvoice is responsible for returning the current status of an invoice with the id in the URL
func (t PaymentAPIService) getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the id is the address of the invoice
	id := p.ByName("id")
	if id == "" {
		fmt.Fprintf(w, "error: missing foreignID")
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
	}
	fmt.Fprintf(w, string(b))
}

// createAccount returns the address of the new account with the foreignID in the URL
func (t PaymentAPIService) createAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	foreignID := p.ByName("foreignID")
	if foreignID == "" {
		fmt.Fprintf(w, "error: missing foreignID")
		return
	}
	addr, err := t.api.MakeAccount(foreignID)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
	}
	fmt.Fprintf(w, string(addr))
}

// getAccount returns the public info of the account with the foreignID in the URL
func (t PaymentAPIService) getAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the id is the address of the invoice
	id := p.ByName("foreignID")
	if id == "" {
		fmt.Fprintf(w, "error: missing foreignID")
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
	}
	fmt.Fprintf(w, string(b))
}

// getAccountByAddress returns the public info of the account with the address in the URL
func (t PaymentAPIService) getAccountByAddress(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the id is the address of the invoice
	id := p.ByName("address")
	if id == "" {
		fmt.Fprintf(w, "error: missing id")
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
	}
	fmt.Fprintf(w, string(b))
}
