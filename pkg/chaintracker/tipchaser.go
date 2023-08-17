package chaintracker

import (
	"context"
	"log"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

const (
	expectedBlockInterval = 90 * time.Second
)

type TipSubscription struct {
	channel  chan<- string
	blocking bool
}

type TipChaser struct {
	// bus             giga.MessageBus
	l1              giga.L1
	ReceiveFromCore chan giga.NodeEvent
	listeners       []TipSubscription
}

/*
 * TipChaser tracks the current Best Block (tip) of the blockchain.
 * It notifies listeners each time the Best Block hash changes.
 * It receives NodeEvent ('Block') from CoreReceiver ZMQ listener.
 * If it doesn't receive ZMQ notifications for a while, it will poll the node instead.
 */
func newTipChaser(conf giga.Config, l1 giga.L1) (*TipChaser, error) {
	result := &TipChaser{
		l1:              l1,
		ReceiveFromCore: make(chan giga.NodeEvent, 1000),
	}
	return result, nil
}

func (c *TipChaser) Subscribe(ch chan<- string, blocking bool) {
	c.listeners = append(c.listeners, TipSubscription{ch, blocking})
}

func (c *TipChaser) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		started <- true
		var lastid string
		for {
			select {
			case <-stop:
				stopped <- true
				return
			case e := <-c.ReceiveFromCore:
				switch e.Type {
				case giga.Block:
					blockid := e.ID
					if blockid != lastid {
						lastid = blockid
						c.sendEvent(blockid)
					}
				}
			case <-time.After(expectedBlockInterval):
				log.Println("TipChaser: falling back to getbestblockhash")
				// c.bus.Send(giga.SYS_ERR, "TipChaser: falling back to getbestblockhash")
				blockid, err := c.l1.GetBestBlockHash()
				if err != nil {
					log.Println("TipChaser: core RPC request failed: getbestblockhash")
					// c.bus.Send(giga.SYS_ERR, "TipChaser: core RPC request failed: getbestblockhash")
				} else {
					if blockid != lastid {
						lastid = blockid
						c.sendEvent(blockid)
					}
				}
			}
		}
	}()

	return nil
}

func (c *TipChaser) sendEvent(e string) {
	log.Println("TipChaser: discovered new best block:", e)
	for _, ch := range c.listeners {
		if ch.blocking {
			ch.channel <- e
		} else {
			// non-blocking send.
			select {
			case ch.channel <- e:
			default:
			}
		}
	}
}
