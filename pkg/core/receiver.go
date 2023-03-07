package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"syscall"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/pebbe/zmq4"
)

// interface guard ensures ZMQEmitter implements giga.NodeEmitter
var _ giga.NodeEmitter = &CoreReceiver{}

// CoreReceiver receives ZMQ messages from Dogecoin Core.
// CAUTION: the protocol is not authenticated!
// CAUTION: subscribers MUST validate the received data since it may be out of date, incomplete or even invalid (fake)
type CoreReceiver struct {
	bus         giga.MessageBus
	sock        *zmq4.Socket
	listeners   []chan<- giga.NodeEvent
	nodeAddress string
}

func (e *CoreReceiver) Subscribe(ch chan<- giga.NodeEvent) {
	e.listeners = append(e.listeners, ch)
}

func NewCoreReceiver(bus giga.MessageBus, config giga.Config) (*CoreReceiver, error) {
	return &CoreReceiver{
		bus:         bus,
		listeners:   make([]chan<- giga.NodeEvent, 0, 10),
		nodeAddress: fmt.Sprintf("tcp://%s:%d", config.Dogecoind[config.Gigawallet.Dogecoind].Host, config.Dogecoind[config.Gigawallet.Dogecoind].ZMQPort),
	}, nil
}

func (z CoreReceiver) Run(started, stopped chan bool, stop chan context.Context) error {
	sock, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		return err
	}
	sock.SetRcvtimeo(2 * time.Second)
	z.sock = sock
	z.bus.Send(giga.SYS_STARTUP, fmt.Sprintf("ZMQ: connecting to: %s", z.nodeAddress))
	err = sock.Connect(z.nodeAddress)
	if err != nil {
		return err
	}
	err = subscribeAll(sock, "hashtx", "rawtx", "hashblock")
	if err != nil {
		return err
	}
	go func() {
		started <- true

		for {
			// Handle shutdown
			select {
			case <-stop:
				sock.Close()
				close(stopped)
				return
			default:
				// fall through to zmq recv
			}

			msg, err := z.sock.RecvMessageBytes(0)
			if err != nil {
				switch err := err.(type) {
				case zmq4.Errno:
					if err == zmq4.Errno(syscall.ETIMEDOUT) {
						// handle timeouts by looping again
						z.bus.Send(giga.SYS_ERR, "ZMQ: connection timeout")
						continue
					} else if err == zmq4.Errno(syscall.EAGAIN) {
						continue
					} else {
						// handle other ZeroMQ error codes
						z.bus.Send(giga.SYS_ERR, fmt.Sprintf("ZMQ err: %s", err))
						continue
					}
				default:
					// handle other Go errors
					panic(fmt.Sprintf("zmq error: %v\n", err))
				}
			}
			tag := string(msg[0])
			switch tag {
			case "hashtx":
				id := toHex(msg[1])
				msg, err = z.sock.RecvMessageBytes(0)
				if err != nil {
					panic(fmt.Sprintf("zmq error: (hashtx %s): %v\n", id, err.Error()))
				}
				if string(msg[0]) != "rawtx" {
					panic(fmt.Sprintf("zmq error: expected rawtx after hashtx %s", id))
				}
				rawtx := toHex(msg[1])
				// fmt.Printf("ZMQ=> TX id=%s rawtx=%s\n", id, rawtx)
				fmt.Printf("ZMQ=> TX id=%s\n", id)
				z.notify(giga.TX, id, rawtx)
			case "hashblock":
				id := toHex(msg[1])
				fmt.Printf("ZMQ=> BLOCK id=%s\n", id)
				z.notify(giga.Block, id, "")
			default:
				fmt.Printf("ZMQ=> %s ??\n", tag)
			}
		}

	}()
	return nil
}

func (z CoreReceiver) notify(tag giga.NodeEventType, id string, data string) {
	e := giga.NodeEvent{
		Type: tag, ID: id, Data: data,
	}
	for _, ch := range z.listeners {
		ch <- e
	}
}

func toHex(b []byte) string {
	return hex.EncodeToString(b)
}

func subscribeAll(sock *zmq4.Socket, topics ...string) error {
	for _, topic := range topics {
		err := sock.SetSubscribe(topic)
		if err != nil {
			return err
		}
	}
	return nil
}
