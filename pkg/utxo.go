package giga

import "github.com/shopspring/decimal"

// UTXO is an Unspent Transaction Output, i.e. a prior payment into our Account.
// This is used in the interface to Store and L1 (libdogecoin)
type UTXO struct {
	TxID          string     // Dogecoin Transaction ID - part of unique key (from Txn Output)
	VOut          int        // Transaction VOut number - part of unique key (from Txn Output)
	Value         CoinAmount // Amount of Dogecoin available to spend (from Txn Output)
	ScriptType    ScriptType // 'p2pkh' etc, see ScriptType constants
	ScriptAddress Address    // P2PKH address required to spend this UTXO (extracted from the script code)
	Account       Address    // Account ID (by searching for ScriptAddress using FindAccountForAddress)
	KeyIndex      uint32     // Account HD Wallet key-index of the ScriptAddress (needed to spend)
	IsInternal    bool       // Account HD Wallet internal/external address flag for ScriptAddress (needed to spend)
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

func (b *TxnBuilder) regenerateTxn() error {
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
	// UTXO must be associated with an Account and ScriptAddress so we can spend it.
	if utxo.Account == "" || utxo.ScriptAddress == "" || utxo.Value.LessThanOrEqual(ZeroCoins) {
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
		ScriptType:    scriptTypeP2PKH,
		Amount:        amount,
		ScriptAddress: payTo,
	})
	return nil
}

// Calculate the minimum Fee payable to mine the transaction.
// The fee is based on the transacion size (i.e. number of inputs and outputs)
func (b *TxnBuilder) calculateFeeForSize() (CoinAmount, error) {
	err := b.regenerateTxn()
	if err != nil {
		return ZeroCoins, err
	}
	numBytes := decimal.NewFromInt(int64(len(b.txn.TxnHex) / 2))
	fee := TxnFeePerByte.Mul(numBytes)
	return fee, nil
}

// Calculate the Fee based on the size of the final signed transaction.
// Make sure the UTXO Inputs cover that fee as well as all Outputs,
// and add new UTXOs to cover the fee if necessary (if this happens,
// the transaction size changes and we need to loop and go again.)
func (b *TxnBuilder) CalculateFee(extraFee CoinAmount) error {
	for {
		sizeFee, err := b.calculateFeeForSize()
		if err != nil {
			return err
		}
		newTotal := b.TotalOutputs().Add(sizeFee).Add(extraFee)
		numInputs := len(b.inputs)
		err = b.AddUTXOsUpToAmount(newTotal)
		if err != nil {
			return err
		}
		if len(b.inputs) > numInputs {
			// Number of inputs changed in order to pay the fee amount.
			// This changes the size of the transaction, so go back and calculate again.
			continue
		}
		// Done: current set of inputs covers the current fee.
		b.fee = sizeFee
		return nil
	}
}

func (b *TxnBuilder) GetFinalTxn() (NewTxn, error) {
	if len(b.txn.TxnHex) > 0 && b.fee.GreaterThan(ZeroCoins) {
		return b.txn, nil
	}
	return NewTxn{}, NewErr(InvalidTxn, "fee has not been calculated yet")
}

// Implement UTXOSet interface.
func (b *TxnBuilder) Includes(txID string, vOut int) bool {
	return b.used[UTXOKey{TxID: txID, VOut: vOut}]
}
