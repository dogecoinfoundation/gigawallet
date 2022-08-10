package giga

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// implements tjstebbing/conductor.Service
type PaymentAPIService struct {
	srv *http.Server
}

func (t PaymentAPIService) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		mux := httprouter.New()
		mux.POST("/invoice", createInvoice)
		mux.GET("/invoice/:id", getInvoice)

		t.srv = &http.Server{Addr: ":8080", Handler: mux}
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

/* getInvoice is responsible for returning the current status of an invoice
 *
 */
func getInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, "invoice for %s!\n", p.ByName("id"))
}

/* createInvoice accepts an invoice structure and returns an Invoice
 *
 */
func createInvoice(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, "create invoice")
}
