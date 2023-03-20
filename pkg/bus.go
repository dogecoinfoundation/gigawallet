package giga

/*
The message subsystem exists to allow event-based access to
the various parts of GigaWallet's processes, for integration purposes.

A simple internal 'message bus' is passed around internally as a
singleton, with an internal goroutine and a 'send' method for sending
'messages'.

outbound destinations are created in config, which result in these
messages being routed to various external services, ie: MQTT, AMQP,
HTTP callbacks, log-files, etc. These are managed by MessageSubscribers:

MessageSubscribers are registered with the bus and are subscribed via
their own channels along with a list of EventTypes they want to subscrive
to.
*/

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

// MessageSubscribers are things that subscribe to the bus and handle
// messages, ie: MQTT, AMQP, http callbacks etc.
type MessageSubscriber interface {
	GetChan() chan Message
}

// Created by the bus, wraps message sent with Send
type Message struct {
	EventType EventType
	Message   []byte
	ID        string // optional
}

type Subscription struct {
	dest  MessageSubscriber
	types []EventType
}

func NewMessageBus() MessageBus {
	return MessageBus{
		receivers: make(map[*Subscription]bool),
		inbound:   make(chan Message, 1),
	}
}

type MessageBus struct {
	// Registered MessageSubscribers.
	receivers map[*Subscription]bool

	// Messages from Send(), destinated for MessageSubscribers
	inbound chan Message
}

// Send a message to the bus with a specific EventType
// msg can be anything JSON serialisable, this will be
// turned into a Message and delivered to any interested MessageSubscribers
func (b MessageBus) Send(t EventType, msg interface{}, msgID ...string) error {
	j, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if len(msgID) == 0 {
		b.inbound <- Message{t, j, generateID()}
	} else {
		b.inbound <- Message{t, j, msgID[0]}
	}
	return nil
}

func (b MessageBus) Register(m MessageSubscriber, types ...EventType) {
	sub := Subscription{m, types}
	b.receivers[&sub] = true
}

func (b MessageBus) Unregister(sub *Subscription) {
	delete(b.receivers, sub)
	close((*sub).dest.GetChan())
}

// Implements conductor Service
func (b MessageBus) Run(started, stopped chan bool, stop chan context.Context) error {

	go func() {
		stopBus := make(chan bool)
		go func() {
			for {
				select {
				case <-stopBus:
					return
				case message := <-b.inbound:
					for sub := range b.receivers {
						// check if this receiver wants this message type
						cont := false
						for _, t := range (*sub).types {
							if t.Type() == "ALL" {
								cont = true
								break
							}
							if t.Type() == message.EventType.Type() {
								cont = true
							}
						}
						if !cont {
							break
						}

						// send the message to the receiver
						select {
						case (*sub).dest.GetChan() <- message:
						default:
							// if we are unable to send, cansel the sub
							b.Send(SYS_ERR, struct{ msg string }{msg: "reciever failed to handle msg, closing"})
							b.Unregister(sub)
						}
					}
				}
			}
		}()

		started <- true
		// wait for shutdown.
		<-stop
		// do some shutdown stuff then signal we're done
		close(stopBus)
		stopped <- true
	}()
	return nil
}

// create a short random ID for msgs that have none
func generateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:8]
}
