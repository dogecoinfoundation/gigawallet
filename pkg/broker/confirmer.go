package broker

import (
	"context"
	"strings"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

type TxnConfirmer struct {
	ReceiveFromNode     chan giga.NodeEvent
	ReceiveFromBroker   chan BrokerEvent
	listeners           []chan<- BrokerEvent
	confirmationsNeeded int
}

/*
 * Confirmer receives giga.NodeEvent from ZMQEmitter
 * Confirmer receives invoice_ids from Store
 * Confirmer receives txn_ids or invoice_ids from Store ???
 * Confirmer sends
 */
func NewTxnConfirmer(conf giga.Config) (*TxnConfirmer, error) {
	result := &TxnConfirmer{ReceiveFromNode: make(chan giga.NodeEvent, 100), confirmationsNeeded: conf.Gigawallet.ConfirmationsNeeded}
	return result, nil
}

func (c *TxnConfirmer) Subscribe(ch chan<- BrokerEvent) {
	c.listeners = append(c.listeners, ch)
}

func (c *TxnConfirmer) Run(started, stopped chan bool, stop chan context.Context) error {
	type txInfo struct {
		id            string
		foundInBlock  bool
		confirmations int
	}

	rawTxToInfo := make(map[string]*txInfo)
	txWatchlist := make(map[string]bool)

	go func() {
		started <- true
		for {
			select {
			case e := <-c.ReceiveFromBroker:
				switch e.Type {
				case NewInvoice:
					txWatchlist[e.ID] = true
				}
			case e := <-c.ReceiveFromNode:
				switch e.Type {
				case giga.TX:
					if txWatchlist[e.ID] {
						rawTxToInfo[e.Data] = &txInfo{id: e.ID, foundInBlock: false, confirmations: 0}
					}
				case giga.Block:
					for raw, info := range rawTxToInfo {
						if !info.foundInBlock {
							if strings.Contains(e.Data, raw) {
								info.foundInBlock = true // next if statement will increment confirmations
							}
						}
						if info.foundInBlock {
							info.confirmations++
						}
						if info.confirmations >= c.confirmationsNeeded {
							e := BrokerEvent{Type: InvoiceConfirmed, ID: info.id}
							for _, ch := range c.listeners {
								ch <- e
							}
							delete(rawTxToInfo, raw)
						}
					}
				}
			case <-stop:
				stopped <- true
				return
			}
		}
	}()

	return nil
}
