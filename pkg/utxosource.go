package giga

import "github.com/dogecoinfoundation/gigawallet/pkg/doge"

// Store UTXO Source used to find UTXOs to spend.
type StoreUTXOSource struct {
	store     Store
	accountID Address
	unspent   []UTXO
	noMore    bool
}

var _ UTXOSource = &StoreUTXOSource{}

func NewUTXOSource(store Store, accountID Address) UTXOSource {
	return &StoreUTXOSource{
		store:     store,
		accountID: accountID,
	}
}

func NewArrayUTXOSource(utxos []UTXO) UTXOSource {
	// Used for tests: because noMore is true, it will not access the store.
	return &StoreUTXOSource{
		store:     nil,
		accountID: "",
		unspent:   utxos,
		noMore:    true,
	}
}

func (s *StoreUTXOSource) fetchMoreUTXOs() error {
	utxos, err := s.store.GetAllUnreservedUTXOs(s.accountID)
	if err != nil {
		return err
	}
	// FIXME: currently GetAllUnreservedUTXOs gets everything at once,
	// but UTXOSource is designed to fetch a few UTXOs at a time.
	s.noMore = true
	s.unspent = append(s.unspent, utxos...)
	if len(utxos) < 1 {
		s.noMore = true // no more UTXOs in account.
	}
	return nil
}

func (s *StoreUTXOSource) NextUnspentUTXO(taken UTXOSet) (UTXO, error) {
	for {
		for _, utxo := range s.unspent {
			if utxo.ScriptType == doge.ScriptTypeP2PKH {
				// Exclude UTXOs that have already been taken from the source.
				if !taken.Includes(utxo.TxID, utxo.VOut) {
					return utxo, nil // found matching UTXO.
				}
			}
		}
		if !s.noMore {
			err := s.fetchMoreUTXOs()
			if err != nil {
				return UTXO{}, err // error fetching UTXOs.
			}
			continue
		}
		return UTXO{}, NewErr(InsufficientFunds, "not enough funds in account")
	}
}

func (s *StoreUTXOSource) FindUTXOLargerThan(amount CoinAmount, taken UTXOSet) (UTXO, error) {
	for {
		for _, utxo := range s.unspent {
			if utxo.ScriptType == doge.ScriptTypeP2PKH {
				// We can (presumably) spend this UTXO with one of our private keys,
				// otherwise it wouldn't be in our account.
				if utxo.Value.GreaterThanOrEqual(amount) {
					// Exclude UTXOs that have already been taken from the source.
					if !taken.Includes(utxo.TxID, utxo.VOut) {
						return utxo, nil // found matching UTXO.
					}
				}
			}
		}
		if !s.noMore {
			err := s.fetchMoreUTXOs()
			if err != nil {
				return UTXO{}, err // error fetching UTXOs.
			}
			continue
		}
		return UTXO{}, NewErr(InsufficientFunds, "not enough funds in account")
	}
}
