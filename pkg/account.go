package giga

import "log"

// The number of addresses HD Wallet discovery will scan beyond the last-used address.
const HD_DISCOVERY_RANGE = 20

// Account is a single user account (Wallet) managed by Gigawallet.
/*
 -- Payouts
	 - PayoutAddress is a dogecoin address to pay to
	 - PayoutThreshold, if non-zero, auto-payout if balance is greater
	 - PayoutFrequency, if set, payout at this schedule
*/
type Account struct {
	Address          Address // HD Wallet master public key as a dogecoin address (Account ID)
	Privkey          Privkey // HD Wallet master extended private key.
	ForeignID        string  // unique identifier supplied by the organisation using Gigawallet.
	NextInternalKey  uint32  // next internal HD Wallet address to use for txn change outputs.
	NextExternalKey  uint32  // next external HD Wallet address to use for an invoice or pay-to address.
	NextPoolInternal uint32  // next internal HD Wallet address to insert into account_address table.
	NextPoolExternal uint32  // next external HD Wallet address to insert into account_address table.
	PayoutAddress    string
	PayoutThreshold  string
	PayoutFrequency  string
}

// AccountBalance holds the current account balances for an Account.
type AccountBalance struct {
	IncomingBalance CoinAmount // pending coins being received (waiting for Txn to be confirmed)
	CurrentBalance  CoinAmount // current balance available to spend now
	OutgoingBalance CoinAmount // spent funds that are not yet confirmed (waiting for Txn to be confirmed)
}

// Generate and store HD Wallet addresses up to 20 beyond any currently-used addresses.
func (a *Account) UpdatePoolAddresses(tx StoreTransaction, lib L1) error {
	// HD Wallet discovery requires us to detect any transactions on the blockchain
	// that use addresses we haven't used yet (up to 20 beyond any used address)
	// We use an account_address table to track all used and future addresses,
	// so when we receive a new block we can query that table to find the account.
	// ASSUMES: NextExternalKey covers all used external addresses on blockchain.
	externalPoolEnd := a.NextExternalKey + HD_DISCOVERY_RANGE
	internalPoolEnd := a.NextInternalKey + HD_DISCOVERY_RANGE
	firstExternal := a.NextPoolExternal
	if firstExternal < externalPoolEnd {
		numberToAdd := externalPoolEnd - firstExternal
		log.Println("UpdatePoolAddresses: generating", numberToAdd, "new external addresses for", a.ForeignID, "starting at", firstExternal)
		addresses, err := a.GenerateAddresses(lib, firstExternal, numberToAdd, false)
		if err != nil {
			return err
		}
		err = tx.StoreAddresses(a.Address, addresses, firstExternal, false)
		if err != nil {
			return err
		}
	}
	firstInternal := a.NextPoolInternal
	if firstInternal < internalPoolEnd {
		numberToAdd := internalPoolEnd - firstInternal
		log.Println("UpdatePoolAddresses: generating", numberToAdd, "new internal addresses for", a.ForeignID, "starting at", firstInternal)
		addresses, err := a.GenerateAddresses(lib, firstInternal, numberToAdd, true)
		if err != nil {
			return err
		}
		err = tx.StoreAddresses(a.Address, addresses, firstInternal, true)
		if err != nil {
			return err
		}
	}
	// These must be updated in the DB by calling tx.StoreAccount(a)
	a.NextPoolInternal = internalPoolEnd
	a.NextPoolExternal = externalPoolEnd
	return nil
}

// Generate sequential HD Wallet addresses, either external or internal.
func (a *Account) GenerateAddresses(lib L1, first uint32, count uint32, isInternal bool) ([]Address, error) {
	var result []Address
	for addressIndex := first; addressIndex < first+count; addressIndex++ {
		addr, err := lib.MakeChildAddress(a.Privkey, addressIndex, isInternal)
		if err != nil {
			return nil, err
		}
		result = append(result, addr)
	}
	return result, nil
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
