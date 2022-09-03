package giga

import (
	"log"
)

type PaymentBroker struct {
	store     Store
	confirmer *Confirmer
}

func NewPaymentBroker(config Config, ne NodeEmitter, s Store) PaymentBroker {
	result := PaymentBroker{confirmer: NewConfirmer(config, ne), store: s}
	go managePendingInvoices(&result)
	return result
}

func managePendingInvoices(p *PaymentBroker) {
	invoiceChan, err := p.store.GetPendingInvoices()
	if err != nil {
		log.Println("Error getting pending invoices:", err)
		return
	}
	for invoice := range invoiceChan {
		p.confirmer.AfterConfirmation(invoice.TXID, func() {
			err := p.store.MarkInvoiceAsPaid(invoice.ID)
			if err != nil {
				log.Println("error marking invoice with id", invoice.ID, "as paid:", err)
				return
			}
		})
	}
}
