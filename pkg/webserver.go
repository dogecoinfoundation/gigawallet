package giga

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// PaymentAPIService implements tjstebbing/conductor.Service
type PaymentAPIService struct {
	srv  *http.Server
	port string
}

func NewPaymentAPIService(config Config) PaymentAPIService {
	return PaymentAPIService{port: config.PaymentService.Port}
}

func (t PaymentAPIService) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		mux := httprouter.New()
		mux.POST("/invoice", createInvoice)
		mux.POST("/order", createOrder)
		mux.GET("/invoice/:id", getInvoice)

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

func createOrder(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var o Order
	json.NewDecoder(r.Body).Decode(&o)
	// TODO: do something with that order
	fmt.Fprintf(w, "get order")
}

// getInvoice is responsible for returning the current status of an invoice
func getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, "invoice for %s!\n", p.ByName("id"))
}

// createInvoice accepts an invoice structure and returns an Invoice
func createInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, "create invoice")
}
