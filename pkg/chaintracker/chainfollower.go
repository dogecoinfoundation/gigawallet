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
	confirmations    int                          // required number of block confirmations.
	stopping         bool                         // set to exit the main loop.
	SetSync          *giga.ReSyncChainFollowerCmd // pending ReSync command.
}

type ChainPos struct {
	BlockHash     string // last block processed ("" at genesis)
	BlockHeight   int64  // height of last block (0 at genesis)
	NextBlockHash string // optional: if known, else ""
	NextSeq       int64  // next seq-no for account change tracking
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
		ReceiveBestBlock: make(chan string, 1),                // signal that tip has changed.
		Commands:         make(chan any, 10),                  // commands to the service.
		confirmations:    conf.Gigawallet.ConfirmationsNeeded, // to confirm a txn (new UTXOs)
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
				return ChainPos{state.BestBlockHash, state.BestBlockHeight, "", state.NextSeq}
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
				NextSeq:         state.NextSeq,
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
	pos = c.rollBackChainState(cmd.BlockHash, pos)
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
			pos = c.rollBackChainState(lastBlock.PreviousBlockHash, pos)
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
	// 1. Follow the chain forwards up to BATCH_SIZE blocks.
	// If we encounter a fork, stop and take note of the fork-point as well.
	// Do all this before we start a database transaction, because we need to keep
	// transactions as short-running as possible to avoid commit conflicts.
	var startPos ChainPos = pos
	var rollbackFrom string = ""
	var blockCount int = 0
	var txIDs []string
	var changes []UTXOChange
	for pos.NextBlockHash != "" {
		//log.Println("ChainFollower: fetching block:", pos.NextBlockHash)
		block := c.fetchBlock(pos.NextBlockHash)
		if block.Confirmations != -1 {
			// Still on-chain, so update chainstate from block transactions.
			changes, txIDs = c.processBlock(block, changes, txIDs)
			// Progress has been made.
			pos = ChainPos{block.Hash, block.Height, block.NextBlockHash, pos.NextSeq}
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
	// 2. Inside a database transaction,
	// Apply each of the UTXO changes we found in the set of blocks above.
	if blockCount > 0 {
		// Transaction retry loop.
		// This is here to save re-fetching all the blocks in the batch.
		// However, eventually we bail and retry the whole process (in case something else is wrong)
		attempts := 10
		for {
			newPos, err := c.attemptToApplyChanges(changes, txIDs, pos)
			if err == nil {
				pos = newPos // update on success.
				break        // success.
			} else {
				c.sleepForRetry(err, 0) // always delay.
				attempts -= 1
				if attempts > 0 {
					continue // retry now.
				} else {
					return startPos // give up.
				}
			}
		}
	}
	// 3. If a fork-point was found above, roll back chainstate to that point.
	if rollbackFrom != "" {
		pos = c.rollBackChainState(rollbackFrom, pos)
	}
	return pos
}

type AccountMap struct {
	Accounts map[string]int64
	NextSeq  int64
}

func NewAccountMap(nextSeq int64) *AccountMap {
	return &AccountMap{Accounts: make(map[string]int64), NextSeq: nextSeq}
}

func (m *AccountMap) AddIds(ids []string) {
	for _, id := range ids {
		if _, present := m.Accounts[id]; !present {
			m.Accounts[id] = m.NextSeq
			m.NextSeq += 1
		}
	}
}

func (m *AccountMap) Add(id string) {
	if _, present := m.Accounts[id]; !present {
		m.Accounts[id] = m.NextSeq
		m.NextSeq += 1
	}
}

func (c *ChainFollower) attemptToApplyChanges(changes []UTXOChange, txIDs []string, pos ChainPos) (ChainPos, error) {
	accounts := NewAccountMap(pos.NextSeq)
	tx := c.beginStoreTxn()
	err := c.applyUTXOChanges(tx, changes, accounts)
	if err != nil {
		// Unable to complete block processing (already logged) - roll back.
		tx.Rollback()
		return pos, err // retry.
	}
	// Confirm UTXOs as a result of accepting this block.
	// This populates the `spendable_height` field in UTXOs with the current BlockHeight,
	// which flags the UTXOs as spendable by the owner account (we don't allow spending
	// UTXOs in not-yet-confirmed transactions; we treat those as "incoming balance.")
	// Also, marking Invoices paid depends on having their incoming UTXOs confirmed.
	// This is used by InvoiceStamper to send "Partial Payment" events.
	utxoAccounts, err := tx.ConfirmUTXOs(c.confirmations, pos.BlockHeight)
	if err != nil {
		// Unable to complete block processing - roll back.
		log.Println("ChainFollower: ConfirmUTXOs:", err)
		tx.Rollback()
		return pos, err // retry.
	}
	accounts.AddIds(utxoAccounts)
	// Mark Invoices as paid if the sum of their Confirmed UTXOs is at least the invoice total.
	// This records the block-height where we decide the invoice is paid (paid_height)
	// This is used by InvoiceStamper to send "Invoice Paid" events.
	invoiceAccounts, err := tx.MarkInvoicesPaid(pos.BlockHeight)
	if err != nil {
		// Unable to complete block processing - roll back.
		log.Println("ChainFollower: MarkInvoicesPaid:", err)
		tx.Rollback()
		return pos, err // retry.
	}
	accounts.AddIds(invoiceAccounts)
	// Mark payments as on-chain if they match the transaction ids in this block.
	// This records to block-height where we saw the payment happen (paid_height)
	// Once paid_height is populated, ConfirmPayments will monitor for confirmation.
	// This is used by PayMaster to send "Payment Accepted" events.
	paymentAccounts, err := tx.MarkPaymentsOnChain(txIDs, pos.BlockHeight)
	if err != nil {
		// Unable to complete block processing - roll back.
		log.Println("ChainFollower: MarkPaymentsOnChain:", err)
		tx.Rollback()
		return pos, err // retry.
	}
	accounts.AddIds(paymentAccounts)
	// Confirm payments that have been on-chain for `confirmations` blocks.
	// This records the block-height where we decided the payment was confirmed.
	// This is used by PayMaster to send "Payment Confirmed" events.
	confirmAccounts, err := tx.ConfirmPayments(c.confirmations, pos.BlockHeight)
	if err != nil {
		// Unable to complete block processing - roll back.
		log.Println("ChainFollower: ConfirmPayments:", err)
		tx.Rollback()
		return pos, err // retry.
	}
	accounts.AddIds(confirmAccounts)
	// Write the new sequence numbers on all affected accounts.
	// This is used by (multiple) Services to keep track of new account changes.
	err = tx.IncChainSeqForAccounts(accounts.Accounts)
	if err != nil {
		// Unable to complete block processing - roll back.
		log.Println("ChainFollower: IncChainSeqForAccounts:", err)
		tx.Rollback()
		return pos, err // retry.
	}
	// Report affected accounts in the log (useful for now)
	for acct, seq := range accounts.Accounts {
		log.Printf("ChainFollower: account was affected: %s (%v)", acct, seq)
	}
	// We have made forward progress:
	// Update the Best Block in the database (checkpoint for restart)
	log.Println("ChainFollower: commiting chain state:", pos.BlockHash, pos.BlockHeight)
	err = tx.UpdateChainState(giga.ChainState{
		BestBlockHash:   pos.BlockHash,
		BestBlockHeight: pos.BlockHeight,
		NextSeq:         accounts.NextSeq,
	}, false)
	if err != nil {
		log.Println("ChainFollower: UpdateChainState:", err)
		tx.Rollback()
		return pos, err // retry.
	}
	// Commit the entire transaction with all changes in the batch.
	err = tx.Commit()
	if err != nil {
		log.Println("ChainFollower: cannot commit DB transaction:", err)
		return pos, err // retry.
	}
	pos.NextSeq = accounts.NextSeq // after commit.
	return pos, nil
}

func (c *ChainFollower) rollBackChainState(fromHash string, oldPos ChainPos) ChainPos {
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
			pos := ChainPos{block.Hash, block.Height, block.NextBlockHash, oldPos.NextSeq}
			pos.NextSeq = c.rollBackChainStateToPos(pos)
			// Caller needs this block hash and next block hash (if any)
			return pos
		}
	}
}

func (c *ChainFollower) rollBackChainStateToPos(pos ChainPos) int64 {
	log.Println("ChainFollower: rolling back chainstate to height:", pos.BlockHeight)
	// wrap the following in a transaction with retry.
	for {
		tx := c.beginStoreTxn()
		// Roll back chainstate above the specified block height.
		newSeq, err := tx.RevertChangesAboveHeight(pos.BlockHeight, pos.NextSeq)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: RevertUTXOsAboveHeight:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		// Update Best Block in the database (checkpoint for restart)
		err = tx.UpdateChainState(giga.ChainState{
			BestBlockHash:   pos.BlockHash,
			BestBlockHeight: pos.BlockHeight,
			NextSeq:         newSeq,
		}, false)
		if err != nil {
			tx.Rollback()
			log.Println("ChainFollower: UpdateChainState:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		// Commit the entire set of changes transactionally.
		err = tx.Commit()
		if err != nil {
			log.Println("ChainFollower: cannot commit DB transaction:", err)
			c.sleepForRetry(err, 0)
			continue // retry.
		}
		return newSeq // success.
	}
}

// Record UTXO changes found in blocks for transactional commit.
type UTXOTag int

const (
	utxoTagNew = iota
	utxoTagSpent
)

type UTXOChange struct {
	Tag           UTXOTag         // all
	TxID          string          // new, spent
	VOut          int             // new, spent
	Value         giga.CoinAmount // new
	ScriptHex     string          // new
	ScriptType    giga.ScriptType // new
	ScriptAddress giga.Address    // new
	Height        int64           // new, spent
	SpendTxID     string          // spent
}

func (c *ChainFollower) applyUTXOChanges(tx giga.StoreTransaction, changes []UTXOChange, accounts *AccountMap) error {
	for _, utxo := range changes {
		switch utxo.Tag {
		case utxoTagNew:
			// Use an address-to-account index (utxo_account_i) to find the account.
			accountID, keyIndex, isInternal, err := tx.FindAccountForAddress(utxo.ScriptAddress)
			if err == nil {
				err = tx.CreateUTXO(giga.UTXO{
					TxID:          utxo.TxID,
					VOut:          utxo.VOut,
					Value:         utxo.Value,
					ScriptHex:     utxo.ScriptHex,
					ScriptType:    utxo.ScriptType,
					ScriptAddress: utxo.ScriptAddress,
					AccountID:     accountID,
					KeyIndex:      keyIndex,
					IsInternal:    isInternal,
					BlockHeight:   utxo.Height,
				})
				if err != nil {
					log.Println("ChainFollower: CreateUTXO:", err, utxo.TxID, utxo.VOut)
					return err // retry.
				}
				accounts.Add(string(accountID))
			} else {
				if giga.IsNotFoundError(err) {
					// log.Println("ChainFollower: no account matches new UTXO:", txn_id, vout.N)
				} else {
					log.Println("ChainFollower: FindAccountForAddress:", err, utxo.ScriptAddress)
					return err // retry.
				}
			}
		case utxoTagSpent:
			accountID, _, err := tx.MarkUTXOSpent(utxo.TxID, utxo.VOut, utxo.Height, utxo.SpendTxID)
			if err != nil {
				log.Println("ChainFollower: MarkUTXOSpent:", err, utxo.TxID, utxo.VOut)
				return err // retry.
			}
			if accountID != "" {
				log.Println("ChainFollower: marking UTXO spent:", utxo.TxID, utxo.VOut, utxo.Height)
				accounts.Add(accountID)
			}
		}
	}
	return nil
}

func (c *ChainFollower) processBlock(block giga.RpcBlock, changes []UTXOChange, txIDs []string) ([]UTXOChange, []string) {
	log.Println("ChainFollower: processing block", block.Hash, len(block.Tx), block.Height)
	// Insert entirely-new UTXOs that don't exist in the database.
	for _, txn_id := range block.Tx {
		txIDs = append(txIDs, txn_id)
		txn := c.fetchTransaction(txn_id)
		for _, vin := range txn.VIn {
			// Ignore coinbase inputs, which don't spend UTXOs.
			if vin.TxID != "" && vin.VOut >= 0 {
				// Mark this UTXO as spent (at this block height)
				// • Note: a Txn cannot spend its own outputs (but it can spend outputs from previous Txns in the same block)
				// • We only care about UTXOs that match a wallet (i.e. we know which wallet they belong to)
				changes = append(changes, UTXOChange{
					Tag:       utxoTagSpent,
					TxID:      vin.TxID,
					VOut:      vin.VOut,
					Height:    block.Height,
					SpendTxID: txn_id,
				})
			}
		}
		for _, vout := range txn.VOut {
			// Ignore outputs that are not spendable.
			if !vout.Value.IsPositive() {
				// log.Println("ChainFollower: skipping zero-value vout:", txn_id, vout.N, vout.ScriptPubKey.Type)
				continue
			}
			if len(vout.ScriptPubKey.Addresses) == 1 {
				// Create a UTXO associated with the wallet that owns the address.
				scriptType := giga.DecodeCoreRPCScriptType(vout.ScriptPubKey.Type)
				// These script-types always contain a single address.
				// FIXME: re-encode p2pk [and p2sh] as Addresses in a consistent format.
				pkhAddress := giga.Address(vout.ScriptPubKey.Addresses[0])
				changes = append(changes, UTXOChange{
					Tag:           utxoTagNew,
					TxID:          txn_id,
					VOut:          vout.N,
					Value:         vout.Value,
					ScriptHex:     vout.ScriptPubKey.Hex,
					ScriptType:    scriptType,
					ScriptAddress: pkhAddress,
					Height:        block.Height,
				})
			} else if len(vout.ScriptPubKey.Addresses) < 1 {
				log.Println("ChainFollower: skipping no-address vout:", txn_id, vout.N, vout.ScriptPubKey.Type)
			} else {
				log.Println("ChainFollower: skipping multi-address vout:", txn_id, vout.N, vout.ScriptPubKey.Type)
			}
		}
	}
	return changes, txIDs
}

func (c *ChainFollower) beginStoreTxn() (tx giga.StoreTransaction) {
	c.tx = nil
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
