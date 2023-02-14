package broker

import (
	"context"
	"fmt"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/shopspring/decimal"
)

type TxnConfirmer struct {
	l1                  giga.L1
	ReceiveFromNode     chan giga.NodeEvent
	ReceiveFromBroker   chan giga.BrokerEvent
	listeners           []chan<- giga.BrokerEvent
	confirmationsNeeded int
}

/*
 * Confirmer receives giga.NodeEvent from ZMQEmitter
 * Confirmer receives invoice_ids from Store
 * Confirmer receives txn_ids or invoice_ids from Store ???
 * Confirmer sends
 */
func NewTxnConfirmer(conf giga.Config, l1 giga.L1) (*TxnConfirmer, error) {
	result := &TxnConfirmer{
		l1:                  l1,
		ReceiveFromNode:     make(chan giga.NodeEvent, 1000),
		confirmationsNeeded: conf.Gigawallet.ConfirmationsNeeded,
	}
	return result, nil
}

func (c *TxnConfirmer) Subscribe(ch chan<- giga.BrokerEvent) {
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
				case giga.NewInvoice:
					txWatchlist[e.ID] = true
				}
			case e := <-c.ReceiveFromNode:
				switch e.Type {
				case giga.TX:
					if txWatchlist[e.ID] {
						rawTxToInfo[e.Data] = &txInfo{id: e.ID, foundInBlock: false, confirmations: 0}
					}
				case giga.Block:
					c.processBlock(e.ID)
					// for raw, info := range rawTxToInfo {
					// 	if !info.foundInBlock {
					// 		if strings.Contains(e.Data, raw) {
					// 			info.foundInBlock = true // next if statement will increment confirmations
					// 		}
					// 	}
					// 	if info.foundInBlock {
					// 		info.confirmations++
					// 	}
					// 	if info.confirmations >= c.confirmationsNeeded {
					// 		e := giga.BrokerEvent{Type: giga.InvoiceConfirmed, ID: info.id}
					// 		for _, ch := range c.listeners {
					// 			ch <- e
					// 		}
					// 		delete(rawTxToInfo, raw)
					// 	}
					// }
				}
			case <-stop:
				stopped <- true
				return
			}
		}
	}()

	return nil
}

// received a block_id (new block) from the network.
// fetch and decode the block from Core.
func (c *TxnConfirmer) processBlock(blockHash string) {
	block := c.fetchBlock(blockHash)

	// make a Set of all UTXOs in Transaction VOuts
	UTXOs := make(map[string]decimal.Decimal) // Set
	for _, txn_id := range block.Tx {
		txn := c.fetchTransaction(txn_id)
		for _, vout := range txn.VOut {
			if vout.Value.IsPositive() { // greater than zero
				for _, addr := range vout.ScriptPubKey.Addresses {
					if val, ok := UTXOs[addr]; ok {
						UTXOs[addr] = val.Add(vout.Value)
					} else {
						UTXOs[addr] = vout.Value
					}
				}
			}
		}
	}

	fmt.Printf("Block=> hash %s contains %d UTXOs: %v\n", blockHash, len(block.Tx), UTXOs)
}

func (c *TxnConfirmer) fetchBlock(blockHash string) giga.RpcBlock {
	for {
		block, err := c.l1.GetBlock(blockHash)
		if err != nil {
			// back-pressure: we need to try again later.
			fmt.Printf("[!] GetBlock: fetching '%s' : %v\n", blockHash, err)
			time.Sleep(1 * time.Second)
		} else {
			return block
		}
	}
}

func (c *TxnConfirmer) fetchTransaction(txnHash string) giga.RawTxn {
	for {
		txn, err := c.l1.GetTransaction(txnHash)
		if err != nil {
			// back-pressure: we need to try again later.
			fmt.Printf("[!] GetTransaction: fetching '%s' : %v\n", txnHash, err)
			time.Sleep(1 * time.Second)
		} else {
			return txn
		}
	}
}
