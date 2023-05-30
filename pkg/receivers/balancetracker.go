package receivers

import (
	"context"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

type BalanceTracker struct {
	// BalanceTracker receives giga.Message via Rec
	Rec chan giga.Message
}

func NewBalanceTracker() BalanceTracker {
	// create an BalanceTracker
	btr := BalanceTracker{
		make(chan giga.Message, 100),
	}
	return btr
}

// Implements giga.MessageSubscriber
func (l BalanceTracker) GetChan() chan giga.Message {
	return l.Rec
}

// Implements conductor.Service
func (l BalanceTracker) Run(started, stopped chan bool, stop chan context.Context) error {
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
