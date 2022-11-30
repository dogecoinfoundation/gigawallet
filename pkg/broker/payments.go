package broker

import (
	"context"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

type PaymentBroker struct {
	Receive   chan giga.BrokerEvent
	listeners []chan<- giga.BrokerEvent
	store     giga.Store
}

func NewPaymentBroker(config giga.Config, s giga.Store) PaymentBroker {
	return PaymentBroker{Receive: make(chan giga.BrokerEvent, 100), store: s}
}

func (p *PaymentBroker) Subscribe(ch chan<- giga.BrokerEvent) {
	p.listeners = append(p.listeners, ch)
}

func (p PaymentBroker) Run(started, stopped chan bool, stop chan context.Context) error {
	storedInvoices, err := p.store.GetPendingInvoices()
	if err != nil {
		log.Println("Error getting pending invoices:", err)
		return err
	}
	go func() {
		started <- true
		for {
			select {
			case e := <-p.Receive:
				switch e.Type {
				case giga.NewInvoice:
					// from New Invoice API or GetPendingInvoices
					// forward the invoice to the Confirmer [RACE vs ZMQ]
					p.sendEvent(giga.BrokerEvent{Type: giga.NewInvoice, ID: e.ID})
				case giga.InvoiceConfirmed:
					// from Confirmer
					err := p.store.MarkInvoiceAsPaid(giga.Address(e.ID))
					if err != nil {
						log.Println("error marking invoice with id", e.ID, "as paid:", err)
						return
					}
				}
			case e := <-storedInvoices:
				p.sendEvent(giga.BrokerEvent{Type: giga.NewInvoice, ID: string(e.ID)})
			case <-stop:
				stopped <- true
				return
			}
		}
	}()
	return nil
}

func (p PaymentBroker) sendEvent(e giga.BrokerEvent) {
	for _, ch := range p.listeners {
		ch <- e
	}
}
