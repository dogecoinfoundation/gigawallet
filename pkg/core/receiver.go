package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/pebbe/zmq4"
)

// interface guard ensures ZMQEmitter implements giga.NodeEmitter
var _ giga.NodeEmitter = &CoreReceiver{}

type CoreReceiver struct {
	sock        *zmq4.Socket
	listeners   []chan<- giga.NodeEvent
	nodeAddress string
}

func (e *CoreReceiver) Subscribe(ch chan<- giga.NodeEvent) {
	e.listeners = append(e.listeners, ch)
}

func NewCoreReceiver(config giga.Config) (*CoreReceiver, error) {
	return &CoreReceiver{
		listeners:   make([]chan<- giga.NodeEvent, 0, 10),
		nodeAddress: fmt.Sprintf("tcp://%s:%d", config.Dogecoind[config.Gigawallet.Dogecoind].Host, config.Dogecoind[config.Gigawallet.Dogecoind].ZMQPort),
	}, nil
}

func (z CoreReceiver) Run(started, stopped chan bool, stop chan context.Context) error {
	sock, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		return err
	}
	z.sock = sock
	log.Println("ZMQ: connecting to:", z.nodeAddress)
	err = sock.Connect(z.nodeAddress)
	if err != nil {
		return err
	}
	// err = subscribeAll(sock, "hashtx", "rawtx", "rawblock")
	err = sock.SetSubscribe("") // enable all messages.
	if err != nil {
		return err
	}
	go func() {
		started <- true

		go func() {
			for {
				msg, err := z.sock.RecvMessageBytes(0)
				if err != nil {
					panic("zmq error: " + err.Error())
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
					fmt.Printf("ZMQ=> TX id=%s rawtx=%s\n", id, rawtx)
					z.notify(giga.TX, id, rawtx)
				case "hashblock":
					id := toHex(msg[1])
					fmt.Printf("ZMQ=> Block id=%s\n", id)
					z.notify(giga.TX, id, "")
				case "rawblock":
					block := toHex(msg[1])
					fmt.Printf("ZMQ=> Block %s\n", block)
					z.notify(giga.Block, "", block)
				default:
					fmt.Printf("ZMQ=> %s ??\n", tag)
				}
			}
		}()

		for {
			// Handle shutdown
			select {
			case <-stop:
				fmt.Println("CLOSING ZMQ")
				sock.Close()
				stopped <- true
				return
			default:
				// fall through to zmq recv
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
