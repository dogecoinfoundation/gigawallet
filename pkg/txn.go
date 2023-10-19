package giga

import (
	"github.com/shopspring/decimal"
)

var oneHundred = decimal.NewFromInt(100)

// We may need to use addUTXOsUpToAmount during calculateFee (source, inputs, used)
// and we need to generate the tx hex repeatedly (lib, acc, inputs, outputs, changeAddress)
type txState struct {
	lib           L1         // L1 for MakeTransaction
	account       Account    // Account for private key to sign transactions
	inputs        []UTXO     // accumulated tx inputs (from addUTXOsUpToAmount)
	outputs       []NewTxOut // specified tx outputs (from payTo)
	outputSum     CoinAmount // sum specified tx outputs (from payTo)
	used          UTXOSet    // accumulated tx inputs (as a set)
	source        UTXOSource // source of available UTXOs (cached on Account)
	changeAddress Address    // change address for unspent portions of UTXOs
	deductFee     bool       // payTo has DeductFeePercent specified
}

func CreateTxn(payTo []PayTo, fixedFee CoinAmount, acc Account, source UTXOSource, lib L1) (NewTxn, error) {
	outputSum, deductFee, err := sumPayTo(payTo)
	if err != nil {
		return NewTxn{}, err
	}
	changeAddress, err := acc.NextChangeAddress(lib)
	if err != nil {
		return NewTxn{}, err
	}
	state := &txState{
		lib:           lib,
		account:       acc,
		inputs:        []UTXO{},
		outputs:       []NewTxOut{},
		outputSum:     outputSum,
		used:          NewUTXOSet(),
		source:        source,
		changeAddress: changeAddress,
		deductFee:     deductFee,
	}

	err = addUTXOsUpToAmount(outputSum, state)
	if err != nil {
		return NewTxn{}, err
	}
	for _, pay := range payTo {
		err = addOutput(pay.PayTo, pay.Amount, state)
		if err != nil {
			return NewTxn{}, err
		}
	}
	if deductFee {
		newTx, err := calculateAndDeductFee(fixedFee, payTo, state)
		if err != nil {
			return NewTxn{}, err
		}
		return newTx, nil
	} else {
		newTx, err := calculateFee(fixedFee, state)
		if err != nil {
			return NewTxn{}, err
		}
		return newTx, nil
	}
}

func sumPayTo(payTo []PayTo) (CoinAmount, bool, error) {
	total := decimal.Zero
	deduct := decimal.Zero
	for _, pay := range payTo {
		total = total.Add(pay.Amount)
		if pay.Amount.LessThan(TxnDustLimit) {
			return ZeroCoins, false, NewErr(InvalidTxn, "amount is less than dust limit - transaction will be rejected: %s pay to %s", pay.Amount.String(), pay.PayTo)
		}
		if pay.DeductFeePercent.IsNegative() {
			return ZeroCoins, false, NewErr(InvalidTxn, "deduct fee percent cannot be negative: %s pay to %s deduct %s", pay.Amount.String(), pay.PayTo, pay.DeductFeePercent.String())
		} else {
			deduct = deduct.Add(pay.DeductFeePercent)
		}
	}
	deductFee := !deduct.IsZero()
	if deductFee && !deduct.Equals(oneHundred) {
		return ZeroCoins, false, NewErr(InvalidTxn, "deduct fee percentages do not add up to 100")
	}
	return total, deductFee, nil
}

func addUTXOsUpToAmount(amount CoinAmount, state *txState) error {
	current := sumInputs(state.inputs)
	for current.LessThan(amount) {
		utxo, err := state.source.NextUnspentUTXO(state.used)
		if err == nil {
			// add input
			state.inputs = append(state.inputs, utxo)
			state.used.Add(utxo.TxID, utxo.VOut)
			// update current total
			current = current.Add(utxo.Value)
		} else {
			return err
		}
	}
	return nil
}

func sumInputs(inputs []UTXO) CoinAmount {
	total := ZeroCoins
	for _, utxo := range inputs {
		total = total.Add(utxo.Value)
	}
	return total
}

func addOutput(payTo Address, amount CoinAmount, state *txState) error {
	if payTo == "" || amount.LessThanOrEqual(ZeroCoins) {
		return NewErr(InvalidTxn, "invalid transaction output")
	}
	state.outputs = append(state.outputs, NewTxOut{
		ScriptType:    ScriptTypeP2PKH,
		Amount:        amount,
		ScriptAddress: payTo,
	})
	return nil
}

// Calculate the minimum Fee payable to mine the transaction.
// The fee is based on the transacion size (i.e. number of inputs and outputs)
func feeForSize(txHex string) CoinAmount {
	numBytes := decimal.NewFromInt(int64(len(txHex) / 2))
	fee := TxnFeePerByte.Mul(numBytes)
	return fee
}

// Calculate the Fee based on the size of the final signed transaction.
// Make sure the UTXO Inputs cover that fee as well as all Outputs,
// and add new UTXOs to cover the fee if necessary (if this happens,
// the transaction size changes and we need to loop and go again.)
func calculateFee(fixedFee CoinAmount, state *txState) (NewTxn, error) {
	// Iterate until b.txn includes the final (stable) fee calculation.
	fee := ZeroCoins
	numInputs := len(state.inputs)
	attempt := 0
	if fixedFee.IsPositive() { // if > 0
		fee = fixedFee // start with the specified fee.
	}
	for {
		// Build the transaction with the current inputs and fee.
		newTx, err := state.lib.MakeTransaction(state.inputs, state.outputs, fee, state.changeAddress, state.account.Privkey)
		if err != nil {
			return NewTxn{}, err
		}
		// Calculate the fee required for the new transaction size.
		minFee := feeForSize(newTx.TxnHex)
		// Override the minimum fee with the specified fee (if > 0)
		if fixedFee.IsPositive() {
			if fixedFee.LessThan(minFee) {
				return NewTxn{}, NewErr(InvalidTxn, "specified fee %v is less than minimum fee %v", fixedFee, minFee)
			}
			minFee = fixedFee // override with the specified fee.
		}
		// Calculate the total required to cover that fee.
		newTotal := state.outputSum.Add(minFee)
		// Add new transaction inputs if necessary to cover the fee.
		err = addUTXOsUpToAmount(newTotal, state)
		if err != nil {
			return NewTxn{}, err
		}
		// If we added an input, it changes the size of the transaction.
		// If the fee changed, it changes the "change" output (which can also change the size!)
		if len(state.inputs) != numInputs || !minFee.Equals(fee) {
			// Prevent fee oscillation.
			if minFee.LessThan(fee) && len(state.inputs) == numInputs {
				// Min fee got smaller (because the transction got slightly smaller) and the
				// size will often oscillate if we change the fee again; go with the current
				// fee and the current encoded transaction (which includes that fee)
				return newTx, nil
			}
			// Loop again to rebuild the transaction with the new minFee.
			fee = minFee
			numInputs = len(state.inputs)
			attempt += 1
			if attempt > 10 {
				return NewTxn{}, NewErr(InvalidTxn, "too many attempts to find a stable fee")
			}
			continue
		}
		// Done: current set of inputs covers the current fee.
		return newTx, nil
	}
}

// Calculate the Fee based on the size of the final signed transaction.
// Then, subtract the fee from the outputs acccording to PayTo.DeductFeePercent
func calculateAndDeductFee(fixedFee CoinAmount, payTo []PayTo, state *txState) (NewTxn, error) {
	fee := ZeroCoins
	attempt := 0
	outAmounts := copyOutputAmounts(payTo)
	if fixedFee.IsPositive() { // if > 0
		fee = fixedFee // start with the specified fee.
	}
	for {
		// Build the transaction with the current inputs and fee.
		newTx, err := state.lib.MakeTransaction(state.inputs, state.outputs, fee, state.changeAddress, state.account.Privkey)
		if err != nil {
			return NewTxn{}, err
		}
		// Calculate the fee required for the new transaction size.
		minFee := feeForSize(newTx.TxnHex)
		// Override the minimum fee with the specified fee (if > 0)
		if fixedFee.IsPositive() {
			if fixedFee.LessThan(minFee) {
				return NewTxn{}, NewErr(InvalidTxn, "specified fee %v is less than minimum fee %v", fixedFee, minFee)
			}
			minFee = fixedFee // override with the specified fee.
		}
		// Deduct that fee from all outputs as per DeductFeePercent.
		for i, pay := range payTo {
			// ASSUMES 1:1 state.outputs and payTo (set up by caller)
			state.outputs[i].Amount = outAmounts[i].Sub(minFee.Mul(pay.DeductFeePercent.Div(oneHundred)))
		}
		// If the fee changed, it changes the "change" output (which can also change the size!)
		if !minFee.Equals(fee) {
			// Prevent fee oscillation.
			if minFee.LessThan(fee) {
				// Min fee got smaller (because the transction got slightly smaller) and the
				// size will often oscillate if we change the fee again; go with the current
				// fee and the current encoded transaction (which includes that fee)
				return newTx, nil
			}
			// Loop again to rebuild the transaction with the new minFee.
			fee = minFee
			attempt += 1
			if attempt > 10 {
				return NewTxn{}, NewErr(InvalidTxn, "too many attempts to find a stable fee")
			}
			continue
		}
		// Done: current set of inputs covers the current fee.
		return newTx, nil
	}
}

func copyOutputAmounts(payTo []PayTo) (result []CoinAmount) {
	for _, pay := range payTo {
		result = append(result, pay.Amount)
	}
	return
}
