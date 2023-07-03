package giga

import "context"

type TipChaserReceiver = chan NodeEvent

type ChainFollower interface {
	SendCommand(cmd any) // send any of the commands below.
}

/** Re-Sync the ChainFollower from a specific block hash on the blockchain.
 *  This will roll back the chain-state to the specified block (by block-height)
 *  and then follow the block-chain forwards to the current tip.
 */
type ReSyncChainFollowerCmd struct {
	BlockHash string // Block hash to re-sync from.
}

/** Restart the ChainFollower in case it becomes stuck. */
type RestartChainFollowerCmd struct{}

/** Stop the ChainFollower. Ctx can have a timeout. */
type StopChainFollowerCmd struct {
	Ctx context.Context
}

/** Plugins to update an account when there is blockchain activity */
type ChainTrackerPlugin interface {
	// A new transaction has been seen (in a block)
	AccountTxn(tx StoreTransaction, account Account, txn RawTxn)

	// A new UTXO has been seen (in a block)
	AccountUTXO(tx StoreTransaction, account Account, txn UTXO)

	// Final pass over all plugins.
	AccountModified(tx StoreTransaction, account Account)
}
