package broker

import (
	"context"
	"log"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

type ChainFollower struct {
	l1               giga.L1
	ReceiveBestBlock chan string
}

/*
 * ChainFollower walks the blockchain, keeping up with the tip (Best Block)
 *
 * If there's a reorganisation, it will walk backwards to the fork-point,
 * reverting the chainstate as it goes, then walk forwards again on the
 * new best chain until it reaches the new tip.
 *
 * ReceiveBestBlock has capacity 1 because we only need to know that the
 * tip has changed since last time we checked (i.e. dirty flag); we don't
 * care about the actual block hash.
 */
func NewChainFollower(conf giga.Config, l1 giga.L1) (*ChainFollower, error) {
	result := &ChainFollower{
		l1:               l1,
		ReceiveBestBlock: make(chan string, 1), // signal that tip has changed.
	}
	return result, nil
}

func (c *ChainFollower) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		started <- true
		for {
			select {
			case <-stop:
				stopped <- true
				return
			case txid := <-c.ReceiveBestBlock:
				log.Println("ChainFollower: received best block:", txid)
			}
			// ...
		}
	}()

	return nil
}
