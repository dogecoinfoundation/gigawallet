package giga

import "strings"

type Confirmer struct {
	nec         chan NodeEvent
	listeners   []chan<- string
	txWatchlist map[string]bool
}

func NewConfirmer(conf Config, ne NodeEmitter) *Confirmer {
	result := &Confirmer{nec: make(chan NodeEvent, 1)}
	ne.Subscribe(result.nec)
	go notifyConfirmer(result, conf)
	return result
}

func (c *Confirmer) Watch(txid string) {
	c.txWatchlist[txid] = true
}

func notifyConfirmer(result *Confirmer, conf Config) {
	type txInfo struct {
		id            string
		foundInBlock  bool
		confirmations int
	}

	rawTxToInfo := make(map[string]*txInfo, 10)

	for {
		e := <-result.nec
		switch e.Type {
		case TX:
			if result.txWatchlist[e.ID] {
				rawTxToInfo[e.Data] = &txInfo{id: e.ID, foundInBlock: false, confirmations: 0}
			}
		case Block:
			for raw, info := range rawTxToInfo {
				if !info.foundInBlock {
					if strings.Contains(e.Data, raw) {
						info.foundInBlock = true // next if statement will increment confirmations
					}
				}
				if info.foundInBlock {
					info.confirmations++
				}
				if info.confirmations >= conf.Gigawallet.ConfirmationsNeeded {
					for i := range result.listeners {
						result.listeners[i] <- info.id
					}
					delete(rawTxToInfo, raw)
				}
			}
		}
	}
}

func (c *Confirmer) Subscribe(ch chan<- string) {
	c.listeners = append(c.listeners, ch)
}
