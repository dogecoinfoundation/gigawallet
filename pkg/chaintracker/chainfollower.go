package chaintracker

import (
	"context"
	"log"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

const (
	DOGECOIN_GENESIS_BLOCK_HASH = "82bc68038f6034c0596b6e313729793a887fded6e92a31fbdf70863f89d9bea2" // Mainnet 1
	RETRY_DELAY                 = 5 * time.Second
	CONFLICT_DELAY              = 250 * time.Millisecond // quarter second
	BATCH_SIZE                  = 10                     // number of blocks
	BATCH_TIMEOUT               = 30 * time.Second       // DB timeout for a batch
)

type ChainFollower struct {
	l1               giga.L1
	store            giga.Store
	ReceiveBestBlock chan string
	stop             chan context.Context
	stopped          chan bool
}

type ChainPos struct {
	BlockHash     string // last block processed ("" at genesis)
	BlockHeight   int64  // height of last block (0 at genesis)
	NextBlockHash string // optional: if known, else ""
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

		// Recover from panic used to stop the service.
		// We use this to avoid returning a 'stopping' bool from every single function.
		defer func() {
			if r := recover(); r != nil {
				log.Println("ChainFollower: service stopped (shutdown panic received):", r)
			}
		}()

		// Fetch the last processed Best Block hash from the DB (restart point)
		// INVARIANT: the chainstate in our database contains the effects of
		// the Best Block we have stored (and all prior blocks per previousblockhash)
		// We MUST update the chainstate before we update this hash.
		log.Println("ChainFollower: fetching chainstate")
		state := c.fetchChainState()

		// Startup: catch up to the current Best Block (tip)
		var pos ChainPos
		if state.BestBlockHash == "" {
			// No BestBlockHash stored, so we must be starting from scratch.
			// Walk the blockchain from the genesis block.
			pos = ChainPos{"", 0, DOGECOIN_GENESIS_BLOCK_HASH}
		} else {
			// Walk forwards on the blockchain from BestBlockHash until we reach the tip.
			pos = ChainPos{state.BestBlockHash, state.BestBlockHeight, ""}
		}
		pos = c.followChainToTip(pos)

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

			// Walk forwards on the blockchain until we reach the tip.
			pos = c.followChainToTip(pos)
		}
	}()

	return nil
}

func (c *ChainFollower) followChainToTip(pos ChainPos) ChainPos {
	// Make forward progress following the chain, rolling back if we encounter a fork.
	if pos.BlockHash != "" {
		// Check if the last-processed block is still on-chain,
		// and fetch the 'nextblockhash' (if any) from Core's chainstate.
		// This is necessary because, for example:
		// • Our last-processed block may have been the tip (Best Block) and it
		//   had no 'nextblockhash' the last time we fetched it.
		// • We may have just started up and we're well behind the tip; our block
		//   might have been part of a fork when we shut down (or any time, really)
		log.Println("ChainFollower: fetching header:", pos.BlockHash)
		lastBlock := c.fetchBlockHeader(pos.BlockHash)
		if lastBlock.Confirmations != -1 {
			// Still on-chain: resume processing from NextBlockHash.
			pos.NextBlockHash = lastBlock.NextBlockHash // can be ""
		} else {
			// The last block we processed is no longer on-chain, so roll back
			// that block and prior blocks until we find a block that is on-chain.
			pos = c.rollBackChainState(lastBlock.PreviousBlockHash)
		}
	}
	// Walk forwards on the blockchain until we reach the tip.
	// If this encounters a fork along the way, it will interally call rollBackChainState
	// and then resume from the block it returns (as necessary, until it reaches the tip).
	for pos.NextBlockHash != "" {
		pos = c.transactionalRollForward(pos)
		c.checkShutdown() // loops must check for shutdown.
	}
	// We have reached the tip of the blockchain.
	log.Println("ChainFollower: reached the tip of the blockchain:", pos.BlockHash)
	return pos
}

func (c *ChainFollower) transactionalRollForward(pos ChainPos) ChainPos {
	// Within a transaction, follow the chain forwards up to BATCH_SIZE blocks.
	// If we encounter a fork, commit progress then roll back to the fork-point.
	var startPos ChainPos = pos
	var rollbackFrom string = ""
	var blockCount int = 0
	tx := c.beginStoreTxn()
	for pos.NextBlockHash != "" {
		//log.Println("ChainFollower: fetching block:", pos.NextBlockHash)
		block := c.fetchBlock(pos.NextBlockHash)
		if block.Confirmations != -1 {
			// Still on-chain, so update chainstate from block transactions.
			if !c.processBlock(tx, block) {
				// Unable to process the block - roll back.
				tx.Rollback()
				return startPos
			}
			// Progress has been made.
			pos = ChainPos{block.Hash, block.Height, block.NextBlockHash}
			blockCount++
			if blockCount > BATCH_SIZE {
				// Commit our progress every BATCH_SIZE blocks.
				break
			}
			c.checkShutdown() // loops must check for shutdown.
		} else {
			// This block is no longer on-chain: commit progress so far,
			// then roll back until we find a block that is on-chain.
			rollbackFrom = block.PreviousBlockHash
			break
		}
	}
	if blockCount > 0 {
		// We have made forward progress: commit the transaction.
		if !c.commitChainState(tx, pos) {
			// Unable to commit forward progress - roll back.
			return startPos
		}
	} else {
		// No progress made: no need to commit the transaction.
		tx.Rollback()
	}
	if rollbackFrom != "" {
		// We found an off-chain block, so also need to roll back.
		pos = c.rollBackChainState(rollbackFrom)
	}
	return pos
}

func (c *ChainFollower) rollBackChainState(fromHash string) ChainPos {
	log.Println("ChainFollower: rolling back from:", fromHash)
	// Walk backwards along the chain (in Core) to find an on-chain block.
	for {
		// Fetch the block header for the previous block.
		log.Println("ChainFollower: fetching previous header:", fromHash)
		block := c.fetchBlockHeader(fromHash)
		if block.Confirmations == -1 {
			// This block is no longer on-chain, so keep walking backwards.
			fromHash = block.PreviousBlockHash
			c.checkShutdown() // loops must check for shutdown.
		} else {
			// Found an on-chain block: roll back all chainstate above this block-height.
			pos := ChainPos{block.Hash, block.Height, block.NextBlockHash}
			c.rollBackChainStateToPos(pos)
			// Caller needs this block hash and next block hash (if any)
			return pos
		}
	}
}

func (c *ChainFollower) rollBackChainStateToPos(pos ChainPos) {
	maxValidHeight := pos.BlockHeight
	log.Println("ChainFollower: rolling back chainstate to height:", maxValidHeight)
	// wrap the following in a transaction with retry.
	for {
		tx, err := c.store.Begin(BATCH_TIMEOUT)
		if err != nil {
			log.Println("ChainFollower: rollBackChainStateToPos: cannot begin:", err)
			c.sleepForRetry(err)
			continue // retry.
		}
		// Roll back chainstate above maxValidHeight.
		err = tx.RevertUTXOsAboveHeight(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: rollBackChainStateToPos: cannot RevertUTXOsAboveHeight:", err)
			c.sleepForRetry(err)
			continue // retry.
		}
		err = tx.RevertTxnsAboveHeight(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: rollBackChainStateToPos: cannot RevertTxnsAboveHeight:", err)
			c.sleepForRetry(err)
			continue // retry.
		}
		// Update Best Block in the database (checkpoint for restart)
		err = tx.UpdateChainState(giga.ChainState{
			BestBlockHash:   pos.BlockHash,
			BestBlockHeight: pos.BlockHeight,
		})
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: rollBackChainStateToPos: cannot UpdateChainState:", err)
			c.sleepForRetry(err)
			continue // retry.
		}
		err = tx.Commit()
		if err != nil {
			log.Println("ChainFollower: rollBackChainStateToPos: cannot commit DB transaction:", err)
			c.sleepForRetry(err)
			continue // retry.
		}
		return // success.
	}
}

func (c *ChainFollower) commitChainState(tx giga.StoreTransaction, pos ChainPos) (committed bool) {
	// Update Best Block in the database (checkpoint for restart)
	log.Println("ChainFollower: commiting chain state:", pos.BlockHash, pos.BlockHeight)
	err := tx.UpdateChainState(giga.ChainState{
		BestBlockHash:   pos.BlockHash,
		BestBlockHeight: pos.BlockHeight,
	})
	if err != nil {
		tx.Rollback()
		log.Println("ChainFollower: processBlock: cannot UpdateChainState:", err)
		c.sleepForRetry(err)
		return false // retry.
	}
	// Commit the entire transaction with all changes in the batch.
	err = tx.Commit()
	if err != nil {
		log.Println("ChainFollower: processBlock: cannot commit DB transaction:", err)
		c.sleepForRetry(err)
		return false // retry.
	}
	return true // committed.
}

func (c *ChainFollower) processBlock(tx giga.StoreTransaction, block giga.RpcBlock) bool {
	log.Println("ChainFollower: processing block", block.Hash, block.Height)
	// Insert entirely-new UTXOs that don't exist in the database.
	for _, txn_id := range block.Tx {
		txn := c.fetchTransaction(txn_id)
		for _, vin := range txn.VIn {
			// Ignore coinbase inputs, which don't spend UTXOs.
			if vin.TxID != "" && vin.VOut >= 0 {
				// Mark this UTXO as spent (at this block height)
				// • Note: a Txn cannot spend its own outputs (but it can spend outputs from previous Txns in the same block)
				// • We only care about UTXOs that are spendable (i.e. have a positive value and an address/PKH)
				// • We only care about UTXOs that match a wallet (i.e. we know which wallet they belong to)
				// log.Println("ChainFollower: marking UTXO spent", vin.TxID, vin.VOut, block.Height)
				err := tx.MarkUTXOSpent(vin.TxID, vin.VOut, block.Height)
				if err != nil {
					log.Println("ChainFollower: processBlock: cannot mark UTXO in DB:", err, vin.TxID, vin.VOut)
					c.sleepForRetry(err)
					return false // retry.
				}
			}
		}
		for _, vout := range txn.VOut {
			// Ignore outputs that are not spendable.
			if vout.Value.IsPositive() && len(vout.ScriptPubKey.Addresses) > 0 {
				// Create a UTXO associated with the wallet that owns the address.
				scriptType := typeOfScript(vout.ScriptPubKey.Type)
				if scriptType == "p2pkh" || scriptType == "p2sh" || scriptType == "p2pk" {
					// These script-types always contain a single address.
					pkhAddress := giga.Address(vout.ScriptPubKey.Addresses[0])
					// Use an address-to-wallet index (utxo_account_i) to find the wallet.
					accountID, keyIndex, isInternal, err := tx.FindAccountForAddress(pkhAddress)
					if err != nil {
						if giga.IsNotFoundError(err) {
							//log.Println("ChainFollower: no account matches new UTXO:", txn_id, vout.N)
						} else {
							log.Println("ChainFollower: processBlock: cannot query FindAccountForAddress in DB:", err, pkhAddress)
							c.sleepForRetry(err)
							return false // retry.
						}
					} else {
						err = tx.CreateUTXO(txn_id, vout.N, vout.Value, scriptType, pkhAddress, accountID, keyIndex, isInternal, block.Height)
						if err != nil {
							log.Println("ChainFollower: processBlock: cannot create UTXO in DB:", err, txn_id, vout.N)
							c.sleepForRetry(err)
							return false // retry.
						}
					}
				} else {
					log.Println("ChainFollower: unknown script type:", txn_id, vout.N, vout.ScriptPubKey.Type)
				}
			} else {
				log.Println("ChainFollower: no value or no address:", txn_id, vout.N, vout.ScriptPubKey.Type)
			}
		}
	}
	return true
}

func typeOfScript(name string) string {
	if name == "pubkeyhash" {
		return "p2pkh"
	}
	if name == "scripthash" {
		return "p2sh"
	}
	if name == "pubkey" {
		return "p2pk"
	}
	return name
}

func (c *ChainFollower) beginStoreTxn() (tx giga.StoreTransaction) {
	for {
		tx, err := c.store.Begin(BATCH_TIMEOUT)
		if err != nil {
			log.Println("ChainFollower: beginStoreTxn: cannot begin:", err)
			c.sleepForRetry(err)
			continue // retry.
		}
		return tx // store on 'c' for rollback on shutdown?
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

func (c *ChainFollower) fetchChainState() giga.ChainState {
	for {
		state, err := c.store.GetChainState()
		if err != nil {
			if giga.IsNotFoundError(err) {
				return giga.ChainState{} // empty chainstate.
			}
			log.Println("ChainFollower: error retrieving best block:", err)
			c.sleepForRetry(err)
		} else {
			return state
		}
	}
}

func (c *ChainFollower) fetchBlock(blockHash string) giga.RpcBlock {
	for {
		block, err := c.l1.GetBlock(blockHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving block:", err)
			c.sleepForRetry(err)
		} else {
			return block
		}
	}
}

func (c *ChainFollower) fetchBlockHeader(blockHash string) giga.RpcBlockHeader {
	for {
		block, err := c.l1.GetBlockHeader(blockHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving block header:", err)
			c.sleepForRetry(err)
		} else {
			return block
		}
	}
}

func (c *ChainFollower) fetchTransaction(txHash string) giga.RawTxn {
	for {
		txn, err := c.l1.GetTransaction(txHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving transaction:", err)
			c.sleepForRetry(err)
		} else {
			return txn
		}
	}
}

func (c *ChainFollower) sleepForRetry(err error) {
	delay := RETRY_DELAY
	if giga.IsDBConflictError(err) {
		delay = CONFLICT_DELAY
	}
	select {
	case <-c.stop:
		// no work to do, just shut down.
		c.stopped <- true
		panic("stopped") // caught in `Run` method.
	case <-time.After(delay):
		return
	}
}

func (c *ChainFollower) checkShutdown() {
	select {
	case <-c.stop:
		// no work to do, just shut down.
		c.stopped <- true
		panic("stopped") // caught in `Run` method.
	default:
		return
	}
}
