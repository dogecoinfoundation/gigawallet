package chaintracker

import (
	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/dogecoinfoundation/gigawallet/pkg/conductor"
)

func StartChainTracker(c *conductor.Conductor, conf giga.Config, l1 giga.L1, store giga.Store) (*TipChaser, error) {
	// Start the TipChaser service
	tc, err := NewTipChaser(conf, l1)
	if err != nil {
		return nil, err
	}
	c.Service("TipChaser", tc)

	// Start the ChainFollower service
	cf, err := NewChainFollower(conf, l1, store)
	if err != nil {
		return nil, err
	}
	tc.Subscribe(cf.ReceiveBestBlock, false) // non-blocking.
	c.Service("ChainFollower", cf)

	return tc, nil
}
