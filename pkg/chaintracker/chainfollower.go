package chaintracker

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

		// Startup: catch up to the current Best Block (tip)
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

		// Main loop: catch up to the current Best Block (tip) each time it changes.
		for {
			// Wait for Core to signal a new Best Block (new block mined)
			select {
			case <-stop:
				stopped <- true
				return
			case <-c.ReceiveBestBlock:
				log.Println("ChainFollower: received new block signal")
			}

			// Walk forwards on the blockchain from lastBlockProcessed until we reach the tip.
			lastBlockProcessed, stopping = c.walkChainForwards(lastBlockProcessed, false)
			if stopping {
				return // stopped.
			}
		}
	}()

	return nil
}

func (c *ChainFollower) walkChainForwards(lastBlockProcessed string, fromGenesis bool) (string, bool) {
	// Begin a store transaction to contain all of our forward-progress changes.
	tx := c.beginStoreTxn()
	if tx == nil {
		return "", true // stopped.
	}
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
			tx.Rollback()
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
	var blockCount int = 0
	var restartPoint = nextBlockToProcess
	for nextBlockToProcess != "" {
		log.Println("ChainFollower: fetching block:", nextBlockToProcess)
		block, stopping := c.fetchBlock(nextBlockToProcess)
		if stopping {
			tx.Rollback()
			return "", true // stopped.
		}
		if block.Confirmations != -1 {
			// Still on-chain, so update chainstate from block transactions.
			stopping := c.processBlock(tx, block)
			if stopping {
				tx.Rollback()
				return "", true // stopped.
			}
			// Continue processing the next block in the chain.
			lastBlockProcessed = block.Hash
			nextBlockToProcess = block.NextBlockHash // can be ""
			// Loops must check for shutdown before looping.
			if c.checkShutdown() {
				tx.Rollback()
				return "", true // stopped.
			}
			blockCount++
			if blockCount > 100 {
				// Commit our progress every 100 blocks.
				c.commitChainState(tx, block.Hash, block.Height)
			}
		} else {
			// This block is no longer on-chain, so roll back prior blocks until
			// we find a block that is on-chain. First, commit changes so far.
			ok, stopped := c.commitChainState()
			lastBlockProcessed, nextBlockToProcess, stopping = c.rollBackChainState(block.PreviousBlockHash)
			if stopping {
				tx.Rollback()
				return "", true // stopped.
			}
		}
	}
	// We have reached the tip of the blockchain.
	c.commitChainState(tx, block.Hash, block.Height)
	log.Println("ChainFollower: reached the tip of the blockchain:", lastBlockProcessed)
	return lastBlockProcessed, false
}

func (c *ChainFollower) rollBackChainState(previousBlockHash string) (string, string, bool) {
	log.Println("ChainFollower: rolling back from:", previousBlockHash)
	// Walk backwards along the chain to find an on-chain block.
	for {
		// Fetch the block header for the previous block.
		log.Println("ChainFollower: fetching previous header:", previousBlockHash)
		block, stopping := c.fetchBlockHeader(previousBlockHash)
		if stopping {
			return "", "", true // stopped.
		}
		if block.Confirmations == -1 {
			// This block is no longer on-chain, so keep walking.
			previousBlockHash = block.PreviousBlockHash
			// Loops must check for shutdown before looping.
			if c.checkShutdown() {
				return "", "", true // stopped.
			}
		} else {
			// Found an on-chain block: roll back chainstate above this block-height.
			stopping = c.rollBackChainStateToHeight(block.Height, block.Hash)
			if stopping {
				return "", "", true // stopped.
			}
			// Caller needs this block hash and next block hash (if any)
			return block.Hash, block.NextBlockHash, false
		}
	}
}

func (c *ChainFollower) rollBackChainStateToHeight(maxValidHeight int64, blockHash string) bool {
	log.Println("ChainFollower: rolling back chainstate to height:", maxValidHeight)
	// wrap the following in a transaction with retry.
	for {
		tx, err := c.store.Begin()
		if err != nil {
			log.Println("ChainFollower: rollBackChainStateToHeight: cannot begin:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return true // stopped.
			}
			continue // retry.
		}
		// Roll back chainstate above maxValidHeight.
		err = tx.RevertUTXOsAboveHeight(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: rollBackChainStateToHeight: cannot RevertUTXOsAboveHeight:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return true // stopped.
			}
			continue // retry.
		}
		err = tx.RevertTxnsAboveHeight(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: rollBackChainStateToHeight: cannot RevertTxnsAboveHeight:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return true // stopped.
			}
			continue // retry.
		}
		// Update Best Block in the database (checkpoint for restart)
		err = tx.UpdateChainState(giga.ChainState{BestBlockHash: blockHash, BestBlockHeight: maxValidHeight})
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: rollBackChainStateToHeight: cannot UpdateChainState:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return true // stopped.
			}
			continue // retry.
		}
		err = tx.Commit()
		if err != nil {
			log.Println("ChainFollower: rollBackChainStateToHeight: cannot commit:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return true // stopped.
			}
			continue // retry.
		}
		return false // success.
	}
}

func (c *ChainFollower) commitChainState(tx giga.StoreTransaction, lastProcessedBlock string, lastBlockHeight int64) (committed, stopping bool) {
	// Update Best Block in the database (checkpoint for restart)
	err := tx.UpdateChainState(giga.ChainState{
		BestBlockHash:   lastProcessedBlock,
		BestBlockHeight: lastBlockHeight,
	})
	if err != nil {
		tx.Rollback()
		log.Println("ChainFollower: processBlock: cannot UpdateChainState:", err)
		if c.sleepInterrupted(RETRY_DELAY) {
			return false, true // stopped.
		}
		return false, false // retry.
	}
	err = tx.Commit()
	if err != nil {
		log.Println("ChainFollower: processBlock: cannot commit:", err)
		if c.sleepInterrupted(RETRY_DELAY) {
			return false, true // stopped.
		}
		return false, false // retry.
	}
	return true, false // committed.
}

func (c *ChainFollower) processBatch(block giga.RpcBlock) (nextBlock giga.RpcBlock, stopping bool) {
	// wrap the following in a transaction with retry.
	for {
		tx := c.beginStoreTxn()
		if tx == nil {
			return block, true // stopped.
		}
		stopping = c.processBlock(tx, block)
		if stopping {
			tx.Rollback()
			return block, true // stopped.
		}
		// Update Best Block in the database (checkpoint for restart)
		err := tx.UpdateChainState(giga.ChainState{BestBlockHash: block.Hash, BestBlockHeight: block.Height})
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: processBlock: cannot UpdateChainState:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return block, true // stopped.
			}
			continue // retry.
		}
		err = tx.Commit()
		if err != nil {
			log.Println("ChainFollower: processBlock: cannot commit:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return block, true // stopped.
			}
			continue // retry.
		}
		return block, false // success.
	}
}

func (c *ChainFollower) processBlock(tx giga.StoreTransaction, block giga.RpcBlock) (stopping bool) {
	log.Println("ChainFollower: processing block", block.Hash, block.Height)
	// Insert entirely-new UTXOs that don't exist in the database.
	for _, txn_id := range block.Tx {
		txn, stopping := c.fetchTransaction(txn_id)
		if stopping {
			return true // stopped.
		}
		for _, vin := range txn.VIn {
			// Ignore coinbase inputs, which don't spend UTXOs.
			if vin.TxID != "" && vin.VOut >= 0 {
				// Mark this UTXO as spent / remove it from the database.
				// • Note: a Txn cannot spend its own outputs (but it can spend outputs from previous Txns in the same block)
				// • We only care about UTXOs that are spendable (i.e. have a positive value and a single address)
				// • We only care about UTXOs that match a wallet (i.e. we know which wallet they belong to)
				// TODO: finish this...
			}
		}
		for _, vout := range txn.VOut {
			// Ignore outputs that are not spendable.
			if vout.Value.IsPositive() && len(vout.ScriptPubKey.Addresses) > 0 {
				// Gigawallet only knows how to spend these types of UTXOs.
				// These script-types always contain a single address.
				// Normal operation once we have caught up and have thousands of wallets:
				// • new UTXOs need to be associated with a wallet
				//   • delete them if we roll back?
				//   • they go back to mempool, but come back again if we switch back?
				//   • block -> txn -> txid:vout -> wallet.balances
				//                               -> txn (wallet spend) -> wallet.balances
				//   • remember: a block can spend txid:vouts that it also creates!
				// • those that don't match a wallet…
				// Do we keep a UTXO set keyed on txn:idx? (used to validate blocks)
				// Do we keep a UTXO set keyed on address? (used when importing wallets)
				// Do we keep a UTXO set keyed on wallet? (only for existing wallets)
				// Making a payment: need to find all UTXOs for one wallet: wallet-id in UTXOs.
				// Receiving payment: need to match new UTXOs to wallet address-sets
				if vout.ScriptPubKey.Type == "p2pkh" || vout.ScriptPubKey.Type == "p2sh" {
					// Insert a UTXO for each address in the script?
					addresses := uniqueStrings(vout.ScriptPubKey.Addresses)
					addresses = addresses
					// TODO: finish this...
				}
			}
		}
	}
	//c.store.InsertUTXOsIfNew()
	// TODO: insert UTXOs into database.
	return false
}

func (c *ChainFollower) beginStoreTxn() (tx giga.StoreTransaction) {
	for {
		tx, err := c.store.Begin()
		if err != nil {
			log.Println("ChainFollower: beginStoreTxn: cannot begin:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return nil // stopped.
			}
			continue // retry.
		}
		return tx
	}
}

func uniqueStrings(source []string) (result []string) {
	if len(source) == 1 { // common.
		result = source // alias to avoid allocation.
		return
	}
	unique := make(map[string]bool)
	for _, addr := range source {
		if !unique[addr] {
			unique[addr] = true
			result = append(result, addr)
		}
	}
	return
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

func (c *ChainFollower) fetchTransaction(txHash string) (giga.RawTxn, bool) {
	for {
		txn, err := c.l1.GetTransaction(txHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving transaction:", err)
			if c.sleepInterrupted(RETRY_DELAY) {
				return giga.RawTxn{}, true // stopped.
			}
		} else {
			return txn, false
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

func (c *ChainFollower) checkShutdown() bool {
	select {
	case <-c.stop:
		// no work to do, just shut down.
		c.stopped <- true
		return true
	default:
		return false
	}
}
