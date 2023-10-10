package giga

import "github.com/shopspring/decimal"

// UTXO is an Unspent Transaction Output, i.e. a prior payment into our Account.
// This is used in the interface to Store and L1 (libdogecoin)
type UTXO struct {
	TxID          string     // Dogecoin Transaction ID - part of unique key (from Txn Output)
	VOut          int        // Transaction VOut number - part of unique key (from Txn Output)
	Value         CoinAmount // Amount of Dogecoin available to spend (from Txn Output)
	ScriptHex     string     // locking script in this UTXO, hex-encoded
	ScriptType    ScriptType // 'p2pkh' etc, see ScriptType constants (detected from ScriptHex)
	ScriptAddress Address    // P2PKH address required to spend this UTXO (extracted from ScriptHex)
	AccountID     Address    // Account ID (by searching for ScriptAddress using FindAccountForAddress)
	KeyIndex      uint32     // Account HD Wallet key-index of the ScriptAddress (needed to spend)
	IsInternal    bool       // Account HD Wallet internal/external address flag for ScriptAddress (needed to spend)
	BlockHeight   int64      // Block Height of the Block that contains this UTXO (NB. used only when inserting!)
	SpendTxID     string     // TxID of the spending transaction
	PaymentID     int64      // ID of payment in `payment` table (if spent by us)
}

// NewTxOut is an output from a new Txn, i.e. creates a new UTXO.
type NewTxOut struct {
	ScriptType    ScriptType // 'p2pkh' etc, see ScriptType constants
	Amount        CoinAmount // Amount of Dogecoin to pay to the PayTo address
	ScriptAddress Address    // Dogecoin P2PKH Address to receive the funds
}

// Composite Key for map.
type UTXOKey struct {
	TxID string // Transaction ID
	VOut int    // Transaction VOut number
}

// TxnBuilder creates and signs a new Dogecoin Transaction.
type TxnBuilder struct {
	lib     L1               // libdogecoin library interface
	account *Account         // POINTER to Account (uses NextChangeAddress, GenerateAddress, UpdatePoolAddresses)
	source  *UTXOSource      // Source of UTXOs in the Account (cache; accesses the Store)
	used    map[UTXOKey]bool // Account UTXOs already included in inputs (during building)
	inputs  []UTXO           // Input UTXOs to spend in the Txn
	outputs []NewTxOut       // Outputs to create (new UTXOs)
	change  Address          // Change address (new HD Wallet internal key)
	fee     CoinAmount       // DogeCoin amount to pay as a fee (usually derived from transaction size)
	txn     NewTxn           // Encoded Tx Hex from libdogecoin
}

func NewTxnBuilder(account *Account, store Store, lib L1) (TxnBuilder, error) {
	changeAddress, err := account.NextChangeAddress(lib)
	if err != nil {
		return TxnBuilder{}, err
	}
	return TxnBuilder{
		lib:     lib,
		account: account,
		source:  account.GetUTXOSource(store),
		used:    make(map[UTXOKey]bool),
		change:  changeAddress,
	}, nil
}

func (b *TxnBuilder) buildTxn() error {
	txn, err := b.lib.MakeTransaction(b.inputs, b.outputs, b.fee, b.change, b.account.Privkey)
	if err != nil {
		return err
	}
	b.txn = txn
	return nil
}

func (b *TxnBuilder) TotalInputs() CoinAmount {
	totalIn := ZeroCoins
	for _, utxo := range b.inputs {
		totalIn = totalIn.Add(utxo.Value)
	}
	return totalIn
}

func (b *TxnBuilder) TotalOutputs() CoinAmount {
	totalOut := ZeroCoins
	for _, utxo := range b.outputs {
		totalOut = totalOut.Add(utxo.Amount)
	}
	return totalOut
}

func (b *TxnBuilder) AddInput(utxo UTXO) error {
	// UTXO must be associated with an Account and ScriptHex so we can spend it.
	if utxo.AccountID == "" || utxo.ScriptHex == "" || utxo.Value.LessThanOrEqual(ZeroCoins) {
		return NewErr(InvalidTxn, "invalid transaction input")
	}
	if !b.Includes(utxo.TxID, utxo.VOut) {
		b.inputs = append(b.inputs, utxo)
		b.used[UTXOKey{TxID: utxo.TxID, VOut: utxo.VOut}] = true
	}
	return nil
}

func (b *TxnBuilder) AddUTXOsUpToAmount(amount CoinAmount) error {
	current := b.TotalInputs()
	for current.LessThan(amount) {
		utxo, err := b.source.NextUnspentUTXO(b)
		if err == nil {
			b.inputs = append(b.inputs, utxo)
			b.used[UTXOKey{TxID: utxo.TxID, VOut: utxo.VOut}] = true
			current = current.Add(utxo.Value)
		} else {
			return err
		}
	}
	return nil
}

func (b *TxnBuilder) AddOutput(payTo Address, amount CoinAmount) error {
	if payTo == "" || amount.LessThanOrEqual(ZeroCoins) {
		return NewErr(InvalidTxn, "invalid transaction output")
	}
	b.outputs = append(b.outputs, NewTxOut{
		ScriptType:    ScriptTypeP2PKH,
		Amount:        amount,
		ScriptAddress: payTo,
	})
	return nil
}

// Calculate the minimum Fee payable to mine the transaction.
// The fee is based on the transacion size (i.e. number of inputs and outputs)
func (b *TxnBuilder) calculateFeeForSize() (CoinAmount, error) {
	numBytes := decimal.NewFromInt(int64(len(b.txn.TxnHex) / 2))
	fee := TxnFeePerByte.Mul(numBytes)
	return fee, nil
}

// Calculate the Fee based on the size of the final signed transaction.
// Make sure the UTXO Inputs cover that fee as well as all Outputs,
// and add new UTXOs to cover the fee if necessary (if this happens,
// the transaction size changes and we need to loop and go again.)
func (b *TxnBuilder) CalculateFee(specifiedFee CoinAmount) error {
	// Iterate until b.txn includes the final (stable) fee calculation.
	numInputs := len(b.inputs)
	attempt := 0
	if specifiedFee.IsPositive() { // if > 0
		b.fee = specifiedFee // start with the specified fee.
	}
	for {
		// Build the transaction with the current b.inputs and b.fee.
		err := b.buildTxn()
		if err != nil {
			return err
		}
		// Calculate the fee required for the new transaction size.
		minFee, err := b.calculateFeeForSize()
		if err != nil {
			return err
		}
		// Override the minimum fee with the specified fee (if > 0)
		if specifiedFee.IsPositive() {
			if specifiedFee.LessThan(minFee) {
				return NewErr(InvalidTxn, "specified fee %v is less than minimum fee %v", specifiedFee, minFee)
			}
			minFee = specifiedFee // override with the specified fee.
		}
		// Calculate the total required to cover that fee.
		newTotal := b.TotalOutputs().Add(minFee)
		// Add new transaction inputs if necessary to cover the fee.
		err = b.AddUTXOsUpToAmount(newTotal)
		if err != nil {
			return err
		}
		// If we added an input, it changes the size of the transaction.
		// If the fee changed, it changes the "change" output (which can also change the size!)
		if len(b.inputs) != numInputs || !minFee.Equals(b.fee) {
			// Prevent fee oscillation.
			if minFee.LessThan(b.fee) && len(b.inputs) == numInputs {
				// Min fee got smaller (because the transction got slightly smaller) and the
				// size will often oscillate if we change the fee again; go with the current
				// b.fee and the current encoded transaction (which includes that fee)
				return nil
			}
			// Loop again to rebuild the transaction with the new minFee.
			b.fee = minFee
			numInputs = len(b.inputs)
			attempt += 1
			if attempt > 10 {
				return NewErr(InvalidTxn, "too many attempts to find a stable fee")
			}
			continue
		}
		// Done: current set of inputs covers the current fee.
		return nil
	}
}

func (b *TxnBuilder) GetFinalTxn() (NewTxn, CoinAmount, error) {
	if b.fee.LessThanOrEqual(ZeroCoins) {
		return NewTxn{}, ZeroCoins, NewErr(InvalidTxn, "fee has not been calculated yet")
	}
	old_hex := b.txn.TxnHex
	err := b.buildTxn()
	if err != nil {
		return NewTxn{}, ZeroCoins, err
	}
	new_hex := b.txn.TxnHex
	if new_hex != old_hex {
		return NewTxn{}, ZeroCoins, NewErr(InvalidTxn, "txn hex was not updated: "+new_hex+" "+old_hex)
	}
	return b.txn, b.fee, nil
}

// Implement UTXOSet interface.
func (b *TxnBuilder) Includes(txID string, vOut int) bool {
	return b.used[UTXOKey{TxID: txID, VOut: vOut}]
}
