package giga

// Composite key for 'used' Set.
type key struct {
	TxID string // Transaction ID
	VOut int    // Transaction VOut number
}

type UTXOSet struct {
	used map[key]bool
}

func NewUTXOSet() UTXOSet {
	return UTXOSet{
		used: map[key]bool{},
	}
}

func (u *UTXOSet) Add(txID string, vOut int) {
	u.used[key{TxID: txID, VOut: vOut}] = true
}

func (u *UTXOSet) Includes(txID string, vOut int) bool {
	return u.used[key{TxID: txID, VOut: vOut}]
}
