package services

import (
	"context"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

type PayMaster struct {
	// PayMaster receives giga.Message via Rec
	Rec chan giga.Message
}

func NewPayMaster() PayMaster {
	master := PayMaster{
		make(chan giga.Message, 100),
	}
	return master
}

// Implements giga.MessageSubscriber
func (l PayMaster) GetChan() chan giga.Message {
	return l.Rec
}

// Implements conductor.Service
func (l PayMaster) Run(started, stopped chan bool, stop chan context.Context) error {
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
