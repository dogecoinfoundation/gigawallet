package broker

import (
	"context"

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
	// storedInvoices, err := p.store.GetPendingInvoices()
	// if err != nil {
	// 	log.Println("Error getting pending invoices:", err)
	// 	return err
	// }
	go func() {
		started <- true
		for {
			select {
			case e := <-p.Receive:
				switch e.Type {
				case giga.NewInvoice:
					// from New Invoice API or GetPendingInvoices
					// forward the invoice to the Confirmer [RACE vs ZMQ]
					// p.sendEvent(giga.BrokerEvent{Type: giga.NewInvoice, ID: e.ID})
				case giga.InvoiceConfirmed:
					// from Confirmer
					// txn, err := p.store.Begin()
					// XXX This loop needs bettre failure modes!
					// this stuff needs to be buffered, perhaps WAL?
					// if err != nil {
					// 	log.Println("Payment Broker couldn't start txn, mark invoice paid", err)
					// 	return
					// }

					// err = txn.MarkInvoiceAsPaid(giga.Address(e.ID))
					// if err != nil {
					// 	txn.Rollback()
					// 	log.Println("error marking invoice with id", e.ID, "as paid:", err)
					// 	return
					// }

					// err = txn.Commit()
					// if err != nil {
					// 	log.Println("Payment Broker couldn't commit txn, mark invoice paid", err)
					// }
				}
			// case e := <-storedInvoices:
			// 	p.sendEvent(giga.BrokerEvent{Type: giga.NewInvoice, ID: string(e.ID)})
			case <-stop:
				stopped <- true
				return
			}
		}
	}()
	return nil
}

// func (p PaymentBroker) sendEvent(e giga.BrokerEvent) {
// 	for _, ch := range p.listeners {
// 		ch <- e
// 	}
// }
