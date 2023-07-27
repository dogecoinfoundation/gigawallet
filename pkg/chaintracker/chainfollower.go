package chaintracker

import (
	"context"
	"log"
	"time"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

const (
	RETRY_DELAY       = 5 * time.Second        // for RPC and Database errors.
	WRONG_CHAIN_DELAY = 5 * time.Minute        // for "Wrong Chain" error (essentially stop)
	CONFLICT_DELAY    = 250 * time.Millisecond // for Database conflicts (concurrent transactions)
	BLOCKS_PER_COMMIT = 10                     // number of blocks per database commit.
)

type ChainFollower struct {
	l1               giga.L1
	store            giga.Store
	tx               giga.StoreTransaction        // non-nil during a transaction (for cleanup)
	ReceiveBestBlock chan string                  // receive from TipChaser.
	Commands         chan any                     // receive ReSyncChainFollowerCmd etc.
	stopping         bool                         // set to exit the main loop.
	SetSync          *giga.ReSyncChainFollowerCmd // pending ReSync command.
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
func newChainFollower(conf giga.Config, l1 giga.L1, store giga.Store) (*ChainFollower, error) {
	result := &ChainFollower{
		l1:               l1,
		store:            store,
		ReceiveBestBlock: make(chan string, 1), // signal that tip has changed.
		Commands:         make(chan any, 10),   // commands to the service.
	}
	return result, nil
}

func (c *ChainFollower) SendCommand(cmd any) {
	c.Commands <- cmd
}

func (c *ChainFollower) Run(started, stopped chan bool, stop chan context.Context) error {
	go func() {
		// Forward `stop` to the `Commands` channel.
		ctx := <-stop
		c.Commands <- giga.StopChainFollowerCmd{Ctx: ctx}
	}()
	go func() {
		// Loop because the service can be internally restarted.
		started <- true
		for !c.stopping {
			c.serviceMain()
		}
		stopped <- true
	}()
	return nil
}

func (c *ChainFollower) serviceMain() {
	// Recover from panic used to stop or restart the service.
	// We use this to avoid returning a 'stopping' bool from every single function.
	defer func() {
		if r := recover(); r != nil {
			log.Println("ChainFollower: panic received:", r)
		}
		if c.tx != nil {
			// can be left after a panic.
			c.tx.Rollback()
			c.tx = nil
		}
	}()

	// Fetch the last processed Best Block hash from the DB (restart point)
	// INVARIANT: the chainstate in our database contains the effects of
	// the Best Block we have stored (and all prior blocks per previousblockhash)
	// We MUST update the chainstate before we update this hash.
	log.Println("ChainFollower: fetching chainstate")
	pos := c.fetchStartingPos()

	// Execute any pending commands.
	if c.SetSync != nil {
		cmd := c.SetSync
		c.SetSync = nil
		pos = c.setSyncHeight(*cmd, pos)
	}

	// Walk forwards on the blockchain until we reach the tip.
	pos = c.followChainToTip(pos)

	// Main loop: catch up to the current Best Block (tip) each time it changes.
	for {
		// Wait for Core to signal a new Best Block (new block mined)
		// or a Command to arrive.
		select {
		case cmd := <-c.Commands:
			log.Println("ChainFollower: received command")
			switch cmt := cmd.(type) {
			case giga.StopChainFollowerCmd:
				c.stopping = true
				return
			case giga.RestartChainFollowerCmd:
				return
			case giga.ReSyncChainFollowerCmd:
				pos = c.setSyncHeight(cmt, pos)
				// fall through to followChainToTip.
			default:
				log.Println("ChainFollower: unknown command received!")
				continue
			}
		case <-c.ReceiveBestBlock:
			log.Println("ChainFollower: received new block signal")
		}

		// Walk forwards on the blockchain until we reach the tip.
		pos = c.followChainToTip(pos)
	}
}

func (c *ChainFollower) fetchStartingPos() ChainPos {
	// Retry loop for transaction error or wrong-chain error.
	for {
		state := c.fetchChainState()
		rootBlock := c.fetchBlockHash(1)
		if state.BestBlockHash != "" {
			// Resume sync.
			// Make sure we're syncing the same blockchain as before.
			if state.RootHash == rootBlock {
				log.Println("ChainFollower: RESUME SYNC :", state.BestBlockHeight)
				return ChainPos{state.BestBlockHash, state.BestBlockHeight, ""}
			} else {
				log.Println("ChainFollower: WRONG CHAIN!")
				log.Println("ChainFollower: Block#1 we have in DB:", state.RootHash)
				log.Println("ChainFollower: Block#1 on Core Node:", rootBlock)
				log.Println("ChainFollower: Please re-connect to a Core Node running the same blockchain we have in the database, or reset your database tables (please see manual for help)")
				c.sleepForRetry(nil, WRONG_CHAIN_DELAY)
			}
		} else {
			// Initial sync.
			// Start at least 100 blocks back from the current Tip,
			// so we're working with a well-confirmed starting block.
			firstHeight := c.fetchBlockCount() - 100
			if firstHeight < 1 {
				firstHeight = 1
			}
			firstBlockHash := c.fetchBlockHash(firstHeight)
			log.Println("ChainFollower: INITIAL SYNC")
			log.Println("ChainFollower: Block#1 on Core Node:", rootBlock)
			log.Println("ChainFollower: Initial Block Height:", firstHeight)
			// Commit the initial start position to the database.
			// Wrap the following in a transaction with retry.
			tx := c.beginStoreTxn()
			err := tx.UpdateChainState(giga.ChainState{
				RootHash:        rootBlock,
				FirstHeight:     firstHeight,
				BestBlockHash:   firstBlockHash,
				BestBlockHeight: firstHeight,
			}, true)
			if err != nil {
				tx.Rollback()
				log.Println("ChainFollower: fetchStartingPos: UpdateChainState:", err)
				c.sleepForRetry(err, 0)
				continue // retry.
			}
			err = tx.Commit()
			if err != nil {
				log.Println("ChainFollower: fetchStartingPos: cannot commit:", err)
				c.sleepForRetry(err, 0)
				continue // retry.
			}
			// Now start again: should resume sync this time.
			log.Println("ChainFollower: wrote chainstate. ready to resume sync.")
			continue
		}
	}
}

func (c *ChainFollower) setSyncHeight(cmd giga.ReSyncChainFollowerCmd, pos ChainPos) ChainPos {
	hdr := c.fetchBlockHeader(cmd.BlockHash)
	if hdr.Height > pos.BlockHeight {
		// New sync block is after current block.
		log.Println("ChainFollower: ReSync: skipping", hdr.Height-pos.BlockHeight, "blocks")
	} else if hdr.Height < pos.BlockHeight {
		// New sync block is before current block.
		log.Println("ChainFollower: ReSync: rolling back", pos.BlockHeight-hdr.Height, "blocks")
	} else {
		// New sync block equals current block.
		log.Println("ChainFollower: ReSync: matches current block, no changes made.")
		return pos
	}
	// This is correct in both cases: if the new block is after current,
	// nothing will match the rollback queries, it will just update ChainState.
	pos = c.rollBackChainState(cmd.BlockHash)
	return pos
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
	affectedAcconts := make(map[string]any)
	tx := c.beginStoreTxn()
	for pos.NextBlockHash != "" {
		//log.Println("ChainFollower: fetching block:", pos.NextBlockHash)
		block := c.fetchBlock(pos.NextBlockHash)
		if block.Confirmations != -1 {
			// Still on-chain, so update chainstate from block transactions.
			if !c.processBlock(tx, block, affectedAcconts) {
				// Unable to process the block (error already logged) - roll back.
				tx.Rollback()
				return startPos
			}
			// Progress has been made.
			pos = ChainPos{block.Hash, block.Height, block.NextBlockHash}
			blockCount++
			if blockCount > BLOCKS_PER_COMMIT {
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
		// Increment the chain-seq number on affected accounts.
		tx.IncChainSeqForAccounts(mapKeys(affectedAcconts))
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

func mapKeys(m map[string]any) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
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
		tx := c.beginStoreTxn()
		// Increment ChainSeq for all accounts that have UTXOs or Txns above maxValidHeight.
		_, err := tx.IncAccountsAffectedByRollback(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: IncAccountsAffectedByRollback:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		// Roll back chainstate above maxValidHeight.
		err = tx.RevertUTXOsAboveHeight(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: RevertUTXOsAboveHeight:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		err = tx.RevertTxnsAboveHeight(maxValidHeight)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: RevertTxnsAboveHeight:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		// Update Best Block in the database (checkpoint for restart)
		err = tx.UpdateChainState(giga.ChainState{
			BestBlockHash:   pos.BlockHash,
			BestBlockHeight: pos.BlockHeight,
		}, false)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: UpdateChainState:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		err = tx.Commit()
		if err != nil {
			log.Println("ChainFollower: cannot commit DB transaction:", err)
			c.sleepForRetry(err, 0)
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
	}, false)
	if err != nil {
		tx.Rollback()
		log.Println("ChainFollower: UpdateChainState:", err)
		c.sleepForRetry(err, 0)
		return false // retry.
	}
	// Commit the entire transaction with all changes in the batch.
	err = tx.Commit()
	if err != nil {
		log.Println("ChainFollower: cannot commit DB transaction:", err)
		c.sleepForRetry(err, 0)
		return false // retry.
	}
	return true // committed.
}

func (c *ChainFollower) processBlock(tx giga.StoreTransaction, block giga.RpcBlock, affectedAcconts map[string]any) bool {
	log.Println("ChainFollower: processing block", block.Hash, block.Height)
	// Insert entirely-new UTXOs that don't exist in the database.
	for _, txn_id := range block.Tx {
		txn := c.fetchTransaction(txn_id)
		for _, vin := range txn.VIn {
			// Ignore coinbase inputs, which don't spend UTXOs.
			if vin.TxID != "" && vin.VOut >= 0 {
				// Mark this UTXO as spent (at this block height)
				// • Note: a Txn cannot spend its own outputs (but it can spend outputs from previous Txns in the same block)
				// • We only care about UTXOs that match a wallet (i.e. we know which wallet they belong to)
				accountID, _, err := tx.MarkUTXOSpent(vin.TxID, vin.VOut, block.Height)
				if err != nil {
					log.Println("ChainFollower: MarkUTXOSpent:", err, vin.TxID, vin.VOut)
					c.sleepForRetry(err, 0)
					return false // retry.
				}
				if accountID != "" {
					log.Println("ChainFollower: marking UTXO spent:", vin.TxID, vin.VOut, block.Height)
					affectedAcconts[accountID] = nil // insert in affectedAcconts.
				}
			}
		}
		for _, vout := range txn.VOut {
			// Ignore outputs that are not spendable.
			if !vout.Value.IsPositive() {
				log.Println("ChainFollower: skipping zero-value vout:", txn_id, vout.N, vout.ScriptPubKey.Type)
				continue
			}
			if len(vout.ScriptPubKey.Addresses) == 1 {
				// Create a UTXO associated with the wallet that owns the address.
				scriptType := giga.DecodeCoreRPCScriptType(vout.ScriptPubKey.Type)
				// These script-types always contain a single address.
				// FIXME: re-encode p2pk [and p2sh] as Addresses in a consistent format.
				pkhAddress := giga.Address(vout.ScriptPubKey.Addresses[0])
				// Use an address-to-account index (utxo_account_i) to find the account.
				accountID, keyIndex, isInternal, err := tx.FindAccountForAddress(pkhAddress)
				if err != nil {
					if giga.IsNotFoundError(err) {
						// log.Println("ChainFollower: no account matches new UTXO:", txn_id, vout.N)
					} else {
						log.Println("ChainFollower: FindAccountForAddress:", err, pkhAddress)
						c.sleepForRetry(err, 0)
						return false // retry.
					}
				} else {
					tx.CreateUTXO(giga.NewUTXO{
						TxID:        txn_id,
						VOut:        vout.N,
						Value:       vout.Value,
						ScriptType:  scriptType,
						PKHAddress:  pkhAddress,
						AccountID:   accountID,
						KeyIndex:    keyIndex,
						IsInternal:  isInternal,
						BlockHeight: block.Height,
					})
					affectedAcconts[string(accountID)] = nil // insert in affectedAcconts.
					if err != nil {
						log.Println("ChainFollower: CreateUTXO:", err, txn_id, vout.N)
						c.sleepForRetry(err, 0)
						return false // retry.
					}
				}
			} else if len(vout.ScriptPubKey.Addresses) < 1 {
				log.Println("ChainFollower: skipping no-address vout:", txn_id, vout.N, vout.ScriptPubKey.Type)
			} else {
				log.Println("ChainFollower: skipping multi-address vout:", txn_id, vout.N, vout.ScriptPubKey.Type)
			}
		}
	}
	return true
}

func (c *ChainFollower) beginStoreTxn() (tx giga.StoreTransaction) {
	if c.tx != nil {
		// can be left after a panic.
		c.tx.Rollback()
		c.tx = nil
	}
	for {
		tx, err := c.store.Begin()
		if err != nil {
			log.Println("ChainFollower: beginStoreTxn:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		c.tx = tx
		return tx // store on 'c' for rollback on shutdown?
	}
}

// func uniqueStrings(source []string) (result []string) {
// 	if len(source) == 1 { // common.
// 		result = source // alias to avoid allocation.
// 		return
// 	}
// 	unique := make(map[string]bool)
// 	for _, addr := range source {
// 		if !unique[addr] {
// 			unique[addr] = true
// 			result = append(result, addr)
// 		}
// 	}
// 	return
// }

func (c *ChainFollower) fetchChainState() giga.ChainState {
	for {
		state, err := c.store.GetChainState()
		if err != nil {
			if giga.IsNotFoundError(err) {
				return giga.ChainState{} // empty chainstate.
			}
			log.Println("ChainFollower: error retrieving best block:", err)
			c.sleepForRetry(err, 0)
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
			c.sleepForRetry(err, 0)
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
			c.sleepForRetry(err, 0)
		} else {
			return block
		}
	}
}

func (c *ChainFollower) fetchBlockHash(height int64) string {
	for {
		hash, err := c.l1.GetBlockHash(height)
		if err != nil {
			log.Println("ChainFollower: error retrieving block hash:", err)
			c.sleepForRetry(err, 0)
		} else {
			return hash
		}
	}
}

func (c *ChainFollower) fetchBlockCount() int64 {
	for {
		count, err := c.l1.GetBlockCount()
		if err != nil {
			log.Println("ChainFollower: error retrieving block count:", err)
			c.sleepForRetry(err, 0)
		} else {
			return count
		}
	}
}

func (c *ChainFollower) fetchTransaction(txHash string) giga.RawTxn {
	for {
		txn, err := c.l1.GetTransaction(txHash)
		if err != nil {
			log.Println("ChainFollower: error retrieving transaction:", err)
			c.sleepForRetry(err, 0)
		} else {
			return txn
		}
	}
}

func (c *ChainFollower) sleepForRetry(err error, delay time.Duration) {
	if delay == 0 {
		delay = RETRY_DELAY
		if giga.IsDBConflictError(err) {
			delay = CONFLICT_DELAY
		}
	}
	select {
	case cmd := <-c.Commands:
		log.Println("ChainFollower: received command")
		switch cm := cmd.(type) {
		case giga.StopChainFollowerCmd:
			c.stopping = true
			panic("stopped") // caught in `Run` method.
		case giga.RestartChainFollowerCmd:
			panic("stopped") // caught in `Run` method.
		case giga.ReSyncChainFollowerCmd:
			c.SetSync = &cm
			panic("restart") // caught in `Run` method.
		default:
			log.Println("ChainFollower: unknown command received!")
		}
	case <-time.After(delay):
		return
	}
}

func (c *ChainFollower) checkShutdown() {
	select {
	case cmd := <-c.Commands:
		log.Println("ChainFollower: received command")
		switch cm := cmd.(type) {
		case giga.StopChainFollowerCmd:
			c.stopping = true
			panic("stopped") // caught in `Run` method.
		case giga.RestartChainFollowerCmd:
			panic("stopped") // caught in `Run` method.
		case giga.ReSyncChainFollowerCmd:
			c.SetSync = &cm
			panic("restart") // caught in `Run` method.
		default:
			log.Println("ChainFollower: unknown command received!")
		}
	default:
		return
	}
}
