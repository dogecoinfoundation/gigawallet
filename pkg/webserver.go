package giga

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dogecoinfoundation/gigawallet/pkg/dogecoin"
	"github.com/dogecoinfoundation/gigawallet/pkg/store"
	"log"
	"net/http"

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
	}
	err = t.api.StoreInvoice(o)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
	}
	// TODO: do something with that invoice
	fmt.Fprintf(w, "get order")
}

// getInvoice is responsible for returning the current status of an invoice
func (t PaymentAPIService) getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// the id is the address of the invoice
	id := p.ByName("id")
	invoice, err := t.api.GetInvoice(Address(id))
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
	}
	b, err := json.Marshal(invoice)
	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
	}
	fmt.Fprintf(w, string(b))
}
