package chaintracker

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
)

func StartChainTracker(c *conductor.Conductor, conf giga.Config, l1 giga.L1, store giga.Store) (giga.TipChaserReceiver, giga.ChainFollower, error) {
	// Start the TipChaser service
	tc, err := newTipChaser(conf, l1)
	if err != nil {
		return nil, nil, err
	}
	c.Service("TipChaser", tc)

	// Start the ChainFollower service
	cf, err := newChainFollower(conf, l1, store)
	if err != nil {
		return nil, nil, err
	}
	tc.Subscribe(cf.ReceiveBestBlock, false) // non-blocking.
	c.Service("ChainFollower", cf)

	return tc.ReceiveFromCore, cf, nil
}
