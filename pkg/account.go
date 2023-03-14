package giga

// Account is a single user account (Wallet) managed by Gigawallet.
/*
 -- Payouts
	 - PayoutAddress is a dogecoin address to pay to
	 - PayoutThreshold, if non-zero, auto-payout if balance is greater
	 - PayoutFrequency, if set, payout at this schedule
*/

type Account struct {
	Address         Address
	Privkey         Privkey
	ForeignID       string
	NextInternalKey uint32
	NextExternalKey uint32
	PayoutAddress   string
	PayoutThreshold string
	PayoutFrequency string
}

// NextChangeAddress generates the next unused "internal address"
// in the Account's HD-Wallet keyspace. NOTE: since callers don't run
// inside a transaction, concurrent requests can end up paying to the
// same change address (we accept this risk)
func (a *Account) NextChangeAddress(lib L1) (Address, error) {
	keyIndex := a.NextInternalKey
	address, err := lib.MakeChildAddress(a.Privkey, keyIndex, true)
	if err != nil {
		return "", err
	}
	return address, nil
}

// UnreservedUTXOs creates an iterator over UTXOs in this Account that
// have not already been earmarked for an outgoing payment (i.e. reserved.)
// UTXOs are fetched incrementally from the Store, because there can be
// a lot of them. This should iterate in desired spending order.
// NOTE: this does not reserve the UTXOs returned; the caller must to that
// by calling Store.CreateTransaction with the selcted UTXOs - and that may
// fail if the UTXOs have been reserved by a concurrent request. In that case,
// the caller should start over with a new UnreservedUTXOs() call.
func (a *Account) UnreservedUTXOs(s Store) (iter UTXOIterator, err error) {
	// TODO: change this to fetch UTXOs from the Store in batches
	// using a paginated query API.
	allUTXOs, err := s.GetAllUnreservedUTXOs(a.Address)
	if err != nil {
		return &AccountUnspentUTXOs{}, err
	}
	return &AccountUnspentUTXOs{utxos: allUTXOs, next: 0}, nil
}

type AccountUnspentUTXOs struct {
	utxos []UTXO
	next  int
}

func (it *AccountUnspentUTXOs) hasNext() bool {
	return it.next < len(it.utxos)
}
func (it *AccountUnspentUTXOs) getNext() UTXO {
	utxo := it.utxos[it.next]
	it.next++
	return utxo
}

// GetPublicInfo gets those parts of the Account that are safe
// to expose to the outside world (i.e. NOT private keys)
func (a Account) GetPublicInfo() AccountPublic {
	return AccountPublic{Address: a.Address, ForeignID: a.ForeignID}
}

type AccountPublic struct {
	Address         Address `json:"id"`
	ForeignID       string  `json:"foreign_id"`
	PayoutAddress   string  `json:"payout_address"`
	PayoutThreshold string  `json:"payout_threshold"`
	PayoutFrequency string  `json:"payout_frequency"`
}
