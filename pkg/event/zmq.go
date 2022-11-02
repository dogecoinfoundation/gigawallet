package event

import (
	"encoding/hex"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/pebbe/zmq4"
)

// interface guard ensures L1Mock implements giga.L1
var _ giga.NodeEmitter = &ZMQEmitter{}

type ZMQEmitter struct {
	sock      *zmq4.Socket
	listeners []chan<- giga.NodeEvent
}

func (e *ZMQEmitter) Subscribe(ch chan<- giga.NodeEvent) {
	e.listeners = append(e.listeners, ch)
}

func NewZMQEmitter(config giga.Config) (*ZMQEmitter, error) {
	sock, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		return &ZMQEmitter{}, err
	}
	err = sock.Connect("tcp://" + config.Dogecoind[config.Gigawallet.Dogecoind].Host + ":" + config.Dogecoind[config.Gigawallet.Dogecoind].ZMQPort)
	if err != nil {
		return &ZMQEmitter{}, err
	}
	err = subscribeAll(sock, "hashtx", "rawtx", "rawblock")
	if err != nil {
		return &ZMQEmitter{}, err
	}

	result := &ZMQEmitter{sock: sock, listeners: make([]chan<- giga.NodeEvent, 0, 10)}

	go func() {
		for {
			msg, err := sock.RecvMessageBytes(0)
			if err != nil {
				panic("zmq error: " + err.Error())
			}
			e := giga.NodeEvent{}
			switch string(msg[0]) {
			case "hashtx":
				e.Type = giga.TX
				e.ID = toHex(msg[1])
				msg, err = sock.RecvMessageBytes(0)
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
			for _, ch := range result.listeners {
				ch <- e
			}
		}
	}()

	return result, nil
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
