package broker

import (
	"context"
	"log"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

const (
	DOGECOIN_GENESIS_BLOCK_HASH = "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691"
	RETRY_DELAY                 = 5 * time.Second
)

type ChainFollower struct {
	l1               giga.L1
	store            giga.Store
	ReceiveBestBlock chan string
	stop             chan context.Context
	stopped          chan bool
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
func NewChainFollower(conf giga.Config, l1 giga.L1, store giga.Store) (*ChainFollower, error) {
	result := &ChainFollower{
		l1:               l1,
		store:            store,
		ReceiveBestBlock: make(chan string, 1), // signal that tip has changed.
	}
	return result, nil
}

func (c *ChainFollower) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		c.stop, c.stopped = stop, stopped // for helpers.
		started <- true

		// Fetch the last processed Best Block hash from the DB (restart point)
		// INVARIANT: the chainstate in our database contains the effects of
		// the Best Block we have stored (and all prior blocks per previousblockhash)
		// We MUST update the chainstate before we update this hash.
		log.Println("ChainFollower: fetching chainstate")
		state, stopping := c.fetchChainState()
		if stopping {
			return // stopped.
		}

		var lastBlockProcessed string
		if state.BestBlockHash == "" {
			// No BestBlockHash stored, so we must be starting from scratch.
			// Walk the blockchain from the genesis block.
			lastBlockProcessed, stopping = c.walkChainForwards(DOGECOIN_GENESIS_BLOCK_HASH, true)
			if stopping {
				return // stopped.
			}
		} else {
			// Walk forwards on the blockchain from BestBlockHash until we reach the tip.
			lastBlockProcessed, stopping = c.walkChainForwards(state.BestBlockHash, false)
			if stopping {
				return // stopped.
			}
		}

		// Wait for Core to signal a new Best Block (new block mined)
		for {
			select {
			case <-stop:
				stopped <- true
				return
			case <-c.ReceiveBestBlock:
				log.Println("ChainFollower: received new block signal")
			}
			lastBlockProcessed, stopping = c.walkChainForwards(lastBlockProcessed, false)
			if stopping {
				return // stopped.
			}
		}
	}()

	return nil
}

func (c *ChainFollower) walkChainForwards(lastBlockProcessed string, fromGenesis bool) (string, bool) {
	nextBlockToProcess := lastBlockProcessed
	if !fromGenesis {
		// Check if the last-processed block is still on-chain,
		// and fetch the 'nextblockhash' (if any) from Core's chainstate.
		// This is necessary because, for example:
		// • Our last-processed block may have been the tip (Best Block) and it
		//   had no 'nextblockhash' the last time we fetched it.
		// • We may have just started up and we're well behind the tip; our block
		//   might have been part of a fork when we shut down (or any time, really)
		log.Println("ChainFollower: fetching header:", nextBlockToProcess)
		lastBlock, stopping := c.fetchBlockHeader(lastBlockProcessed)
		if stopping {
			return "", true // stopped.
		}
		if lastBlock.Confirmations != -1 {
			// Still on-chain, so begin processing from nextblockhash.
			nextBlockToProcess = lastBlock.NextBlockHash // can be ""
		} else {
			// The last block we processed is no longer on-chain, so roll back
			// that block and prior blocks until we find a block that is on-chain.
			lastBlockProcessed, nextBlockToProcess, stopping = c.rollBackChainState(lastBlock.PreviousBlockHash)
			if stopping {
				return "", true // stopped.
			}
		}
	}
	// Walk forwards on the blockchain from nextBlockToProcess until we reach the tip.
	// nextBlockToProcess can be "" if we're already at the tip.
	// If this encounters a fork along the way, it will interally call rollBackChainState
	// and then resume from the block it returns (as necessary, until it reaches the tip).
	for nextBlockToProcess != "" {
		log.Println("ChainFollower: fetching block:", nextBlockToProcess)
		block, stopping := c.fetchBlock(nextBlockToProcess)
		if stopping {
			return "", true // stopped.
		}
		if block.Confirmations != -1 {
			// Still on-chain, so update chainstate from block transactions.
			stopping := c.processBlock(block)
			if stopping {
				return "", true // stopped.
			}
			// Continue processing the next block in the chain.
			lastBlockProcessed = block.Hash
			nextBlockToProcess = block.NextBlockHash // can be ""
		} else {
			// This block is no longer on-chain, so roll back prior blocks until
			// we find a block that is on-chain.
			lastBlockProcessed, nextBlockToProcess, stopping = c.rollBackChainState(block.PreviousBlockHash)
			if stopping {
				return "", true // stopped.
			}
		}
	}
	// We have reached the tip of the blockchain.
	log.Println("ChainFollower: reached the tip of the blockchain:", lastBlockProcessed)
	return lastBlockProcessed, false
}

func (c *ChainFollower) rollBackChainState(previousBlockHash string) (string, string, bool) {
	log.Println("ChainFollower: rolling back from:", previousBlockHash)
	// TODO: walk upwards to on-chain block.
	// TODO: roll back chainstate above that block-height.
	// TODO: update BestBlockHash in database.
	return previousBlockHash, "", false
}

func (c *ChainFollower) processBlock(block giga.RpcBlock) bool {
	log.Println("ChainFollower: processing block", block.Hash, block.Height)
	// TODO: insert UTXOs into database.
	// TODO: update BestBlockHash in database.
	return false
}

func (c *ChainFollower) fetchChainState() (giga.ChainState, bool) {
	for {
		state, err := c.store.GetChainState()
		if err != nil {
			if giga.IsNotFoundError(err) {
				return giga.ChainState{}, false // empty chainstate.
			}
			log.Println("ChainFollower: error retrieving best block:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return giga.ChainState{}, true // stopped.
			}
		} else {
			return state, false
		}
	}
}

func (c *ChainFollower) fetchBlock(blockHash string) (giga.RpcBlock, bool) {
	for {
		block, err := c.l1.GetBlock(blockHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving block:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return giga.RpcBlock{}, true // stopped.
			}
		} else {
			return block, false
		}
	}
}

func (c *ChainFollower) fetchBlockHeader(blockHash string) (giga.RpcBlockHeader, bool) {
	for {
		block, err := c.l1.GetBlockHeader(blockHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving block header:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return giga.RpcBlockHeader{}, true // stopped.
			}
		} else {
			return block, false
		}
	}
}

func (c *ChainFollower) sleepInterrupted(d time.Duration) bool {
	select {
	case <-c.stop:
		// no work to do, just shut down.
		c.stopped <- true
		return true
	case <-time.After(d):
		return false
	}
}
