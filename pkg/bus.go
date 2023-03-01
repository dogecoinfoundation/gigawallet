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
their own channels along with a list of MessageTypes they want to subscrive
to.
*/

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

type MessageType string

// These consts are used to pub and sub to messages
const (
	MSG_ALL MessageType = "ALL" // Do not use for sending
	MSG_SYS MessageType = "SYS" // System messages
	MSG_NET MessageType = "NET" // Network Events
	MSG_ACC MessageType = "ACC" // Account Events
	MSG_INV MessageType = "INV" // Innvoice Events
)

// slice of all msg types for config funcs lookup
var MSG_TYPES []MessageType = []MessageType{MSG_ALL,
	MSG_SYS, MSG_NET, MSG_ACC, MSG_INV}

// MessageSubscribers are things that subscribe to the bus and handle
// messages, ie: MQTT, AMQP, http callbacks etc.
type MessageSubscriber interface {
	GetChan() chan Message
}

// Created by the bus, wraps message sent with Send
type Message struct {
	MessageType MessageType
	Message     []byte
	ID          string // optional
}

type Subscription struct {
	dest  MessageSubscriber
	types []MessageType
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

// Send a message to the bus with a specific MessageType
// msg can be anything JSON serialisable, this will be
// turned into a Message and delivered to any interested MessageSubscribers
func (b MessageBus) Send(t MessageType, msg interface{}, msgID ...string) error {
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

func (b MessageBus) Register(m MessageSubscriber, types ...MessageType) {
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
							if t == MSG_ALL {
								cont = true
								break
							}
							if t == message.MessageType {
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
							b.Send(MSG_SYS, struct{ msg string }{msg: "reciever failed to handle msg, closing"})
							b.Unregister(sub)
						}
					}
				}
			}
		}()

		started <- true
		select {
		//case ctx := <-stop:
		case <-stop:
			// do some shutdown stuff then signal we're done
			close(stopBus)
			stopped <- true
		}

	}()
	return nil
}

// create a short random ID for msgs that have none
func generateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:8]
}
