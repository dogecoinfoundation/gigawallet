package giga

// Composite key for 'used' Set.
type key struct {
	TxID string // Transaction ID
	VOut int    // Transaction VOut number
}

type UTXOMapSet struct {
	used map[key]bool
}

var _ UTXOSet = UTXOMapSet{}

func NewUTXOSet() UTXOMapSet {
	return UTXOMapSet{
		used: map[key]bool{},
	}
}

func (u UTXOMapSet) Add(txID string, vOut int) {
	u.used[key{TxID: txID, VOut: vOut}] = true
}

func (u UTXOMapSet) Includes(txID string, vOut int) bool {
	return u.used[key{TxID: txID, VOut: vOut}]
}
