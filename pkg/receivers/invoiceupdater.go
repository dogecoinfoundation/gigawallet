package receivers

import (
	"context"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

type InvoiceUpdater struct {
	// InvoiceUpdater receives giga.Message via Rec
	Rec chan giga.Message
}

func NewInvoiceUpdater() InvoiceUpdater {
	// create an InvoiceUpdater
	iup := InvoiceUpdater{
		make(chan giga.Message, 100),
	}
	return iup
}

// Implements giga.MessageSubscriber
func (l InvoiceUpdater) GetChan() chan giga.Message {
	return l.Rec
}

// Implements conductor.Service
func (l InvoiceUpdater) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		started <- true
		for {
			select {
			// handle stopping the service
			case <-stop:
				close(l.Rec)
				close(stopped)
				return
			case msg := <-l.Rec:
				msg = msg
				// l.Log.Printf("%s:%s (%s): %s\n",
				// 	msg.EventType.Type(),
				// 	msg.EventType,
				// 	msg.ID,
				// 	msg.Message)
			}
		}
	}()
	return nil
}
