package receiver

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/pebbe/zmq4"
)

// interface guard ensures ZMQEmitter implements giga.NodeEmitter
var _ giga.NodeEmitter = &ZMQReceiver{}

type ZMQReceiver struct {
	sock        *zmq4.Socket
	listeners   []chan<- giga.NodeEvent
	nodeAddress string
}

func (e *ZMQReceiver) Subscribe(ch chan<- giga.NodeEvent) {
	e.listeners = append(e.listeners, ch)
}

func NewZMQReceiver(config giga.Config) (*ZMQReceiver, error) {
	return &ZMQReceiver{
		listeners:   make([]chan<- giga.NodeEvent, 0, 10),
		nodeAddress: "tcp://" + config.Dogecoind[config.Gigawallet.Dogecoind].Host + ":" + config.Dogecoind[config.Gigawallet.Dogecoind].ZMQPort,
	}, nil
}

func (z ZMQReceiver) Run(started, stopped chan bool, stop chan context.Context) error {
	sock, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		return err
	}
	log.Println("ZMQ: connecting to:", z.nodeAddress)
	err = sock.Connect(z.nodeAddress)
	if err != nil {
		return err
	}
	err = subscribeAll(sock, "hashtx", "rawtx", "rawblock")
	if err != nil {
		return err
	}
	go func() {
		started <- true
		for {
			msg, err := z.sock.RecvMessageBytes(0)
			if err != nil {
				panic("zmq error: " + err.Error())
			}
			e := giga.NodeEvent{}
			switch string(msg[0]) {
			case "hashtx":
				e.Type = giga.TX
				e.ID = toHex(msg[1])
				msg, err = z.sock.RecvMessageBytes(0)
				if err != nil {
					panic("zmq error: " + err.Error())
				}
				if string(msg[0]) != "rawtx" {
					panic("expected rawtx after hashtx")
				}
				e.Data = toHex(msg[1])
			case "rawblock":
				e.Type = giga.Block
				e.Data = toHex(msg[1])
			}
			fmt.Printf("ZMQ=> %+v", e)
			for _, ch := range z.listeners {
				ch <- e
			}
		}
	}()
	return nil
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
