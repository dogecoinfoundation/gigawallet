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

// Mock ChainFollower for tests.
type MockFollower struct {
}

func (m MockFollower) SendCommand(cmd any) {}
