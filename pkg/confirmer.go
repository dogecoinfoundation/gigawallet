package giga

import "strings"

type Confirmer struct {
	nec         chan NodeEvent
	txWatchlist map[string]func()
}

func NewConfirmer(conf Config, ne NodeEmitter) *Confirmer {
	result := &Confirmer{nec: make(chan NodeEvent, 1)}
	ne.Subscribe(result.nec)
	go notifyConfirmer(result, conf.Gigawallet.ConfirmationsNeeded)
	return result
}

// AfterConfirmation calls a function after a transaction is confirmed.
func (c *Confirmer) AfterConfirmation(txid string, funcToCall func()) {
	c.txWatchlist[txid] = funcToCall
}

func notifyConfirmer(result *Confirmer, confirmationsNeeded int) {
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
			if result.txWatchlist[e.ID] != nil {
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
				if info.confirmations >= confirmationsNeeded {
					result.txWatchlist[info.id]() // call the user's function
					delete(rawTxToInfo, raw)
				}
			}
		}
	}
}
