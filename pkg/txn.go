package giga

import (
	"log"

	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	"github.com/shopspring/decimal"
)

// We may need to use addUTXOsUpToAmount during calculateFee (source, inputs, used)
// and we need to generate the tx hex repeatedly (lib, acc, inputs, outputs, changeAddress)
type txState struct {
	lib             L1           // L1 for MakeTransaction
	account         Account      // Account for private key to sign transactions
	inputs          []UTXO       // accumulated tx inputs (from addUTXOsUpToAmount)
	outputs         []NewTxOut   // specified tx outputs (from payTo)
	originalAmounts []CoinAmount // output amounts saved by addOutput (before fee deduction)
	outputSum       CoinAmount   // sum specified tx outputs (from payTo)
	used            UTXOSet      // accumulated tx inputs (as a set)
	source          UTXOSource   // source of available UTXOs (cached on Account)
	changeAddress   Address      // change address for unspent portions of UTXOs
	deductFee       bool         // payTo has DeductFeePercent specified
}

func CreateTxn(payTo []PayTo, fixedFee CoinAmount, maxFee CoinAmount, acc Account, source UTXOSource, lib L1) (NewTxn, error) {
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

	var fee CoinAmount
	if deductFee {
		fee, err = calculateAndDeductFee(fixedFee, maxFee, payTo, state)
		if err != nil {
			return NewTxn{}, err
		}
	} else {
		fee, err = calculateFee(fixedFee, maxFee, state)
		if err != nil {
			return NewTxn{}, err
		}
	}

	// Build the transaction with the current inputs and fee.
	return state.lib.MakeTransaction(state.inputs, state.outputs, fee, state.changeAddress, state.account.Privkey)
}

func sumPayTo(payTo []PayTo) (CoinAmount, bool, error) {
	total := ZeroCoins
	deduct := ZeroCoins
	for _, pay := range payTo {
		total += pay.Amount
		if pay.Amount < TxnDustLimit {
			return ZeroCoins, false, NewErr(InvalidTxn, "PayTo Amount cannot be less than the Dogecoin Dust Limit (%vƉ): The request was to pay %vƉ to %v", TxnDustLimit, pay.Amount, pay.PayTo)
		}
		if pay.DeductFeePercent < 0 {
			return ZeroCoins, false, NewErr(InvalidTxn, "Deduct-Fee-Percent cannot be negative: The request was to deduct %v%% from a payment of %vƉ to %v", pay.DeductFeePercent, pay.Amount, pay.PayTo)
		} else {
			deduct += pay.DeductFeePercent
		}
	}
	if deduct != 0 && deduct != 100 {
		return ZeroCoins, false, NewErr(InvalidTxn, "The requested Deduct-Fee-Percent values do not add up to 100")
	}
	return total, deduct != 0, nil
}

func addUTXOsUpToAmount(amount CoinAmount, state *txState) error {
	current := sumInputs(state.inputs)
	for current < amount {
		utxo, err := state.source.NextUnspentUTXO(state.used)
		if err == nil {
			// add input
			state.inputs = append(state.inputs, utxo)
			state.used.Add(utxo.TxID, utxo.VOut)
			// update current total
			current += utxo.Value
		} else {
			return err
		}
	}
	return nil
}

func sumInputs(inputs []UTXO) CoinAmount {
	total := ZeroCoins
	for _, utxo := range inputs {
		total += utxo.Value
	}
	return total
}

func addOutput(payTo Address, amount CoinAmount, state *txState) error {
	if payTo == "" {
		return NewErr(InvalidTxn, "Invalid transaction output: missing 'to' address in the request.")
	}
	if amount <= ZeroCoins {
		return NewErr(InvalidTxn, "Invalid transaction output: the 'amount' is negative or zero.")
	}
	state.outputs = append(state.outputs, NewTxOut{
		ScriptType:    doge.ScriptTypeP2PKH,
		Amount:        amount,
		ScriptAddress: payTo,
	})
	// also save the amount in originalAmounts for fee calculations (calculateAndDeductFee)
	state.originalAmounts = append(state.originalAmounts, amount)
	return nil
}

// Adjust an output's amount by deducting a percentage of the fee.
// This uses an array of originalAmounts captured by addOutput.
func subtractFeeFromOutput(output int, fee CoinAmount, feePercent CoinAmount, state *txState) error {
	feeAmount := fee * (feePercent / 100) // NB. (feePercent/100) <= 1.0
	if feeAmount > 0 {
		newAmount := state.originalAmounts[output] - feeAmount
		if newAmount < TxnDustLimit {
			// TODO: we may need to offer the option to drop outputs below the dust limit,
			// i.e. skip the payment to a party whose fee contribution consumes the entire payment.
			return NewErr(InvalidTxn,
				"After subtracting the fee percentage, the PayTo Amount is less than the Dogecoin Dust Limit (%vƉ): The request was to pay %vƉ to %v subtracting %v%% of the fee %vƉ which leaves %vƉ remaining.",
				TxnDustLimit, state.originalAmounts[output], state.outputs[output].ScriptAddress,
				feePercent, fee, newAmount)
		}
		state.outputs[output].Amount = newAmount
	}
	return nil
}

// Calculate the size of a P2PKH transaction.
func sizeOfP2PKH(nIn int, nOut int) int64 {
	// inspired by https://bitcoinops.org/en/tools/calc-size/
	// max size for up to 252 inputs and outputs
	return 10 + int64(nIn)*148 + int64(nOut)*34
}

// Get fee estimate from Core `estimatefee` if available,
// otherwise use the base consensus TxnFeePerByte.
func estimateFeePerByte(lib L1) CoinAmount {
	feePerKB, err := lib.EstimateFee(6)
	if err != nil {
		log.Printf("feeForP2PKH: did not use estimatefee due to error: %s", err.Error())
	} else {
		perByte := feePerKB / 1000
		if perByte < TxnFeePerByte {
			perByte = TxnFeePerByte
		}
		return perByte
	}
	return TxnFeePerByte
}

// Calculate fee based on transacion size, within fee limits.
func feeForTxn(sizeBytes int64, feePerByte CoinAmount, fixedFee CoinAmount, maxFee CoinAmount) CoinAmount {
	if fixedFee > 0 {
		return fixedFee // override with specified fee
	}
	feeForSize := feePerByte * sizeBytes
	feeForSize = decimal.Max(feeForSize, TxnRecommendedMinFee) // at least minFee
	return decimal.Min(feeForSize, maxFee)                     // limit to maxFee
}

// Calculate the Fee based on the size of the transaction.
// Make sure the UTXO Inputs cover that fee as well as all Outputs:
// add new UTXOs to cover the fee if necessary (and loop.)
func calculateFee(fixedFee CoinAmount, maxFee CoinAmount, state *txState) (CoinAmount, error) {
	attempt := 0
	feePerByte := estimateFeePerByte(state.lib)
	for {
		// Calculate the fee required for the transaction size.
		sizeBytes := sizeOfP2PKH(len(state.inputs), len(state.outputs)+1) // +1 for Change output
		fee := feeForTxn(sizeBytes, feePerByte, fixedFee, maxFee)
		// Calculate the input total required to cover that fee.
		newTotal := state.outputSum.Add(fee)
		// Add new transaction inputs if necessary to cover the fee.
		prevInputs := len(state.inputs)
		err := addUTXOsUpToAmount(newTotal, state)
		if err != nil {
			return ZeroCoins, err
		}
		// If we added an input, it changes the size of the transaction.
		if len(state.inputs) > prevInputs {
			attempt += 1
			if attempt > 10 {
				return ZeroCoins, NewErr(InvalidTxn, "Too many attempts to find a stable fee (adding inputs to pay for the transaction fee;) 10 attempts were made.")
			}
			continue
		}
		// Done: current set of inputs covers the current fee.
		return fee, nil
	}
}

// Calculate the Fee based on the transaction size, then
// subtract the fee from the outputs acccording to DeductFeePercent
func calculateAndDeductFee(fixedFee CoinAmount, maxFee CoinAmount, payTo []PayTo, state *txState) (CoinAmount, error) {
	// Calculate the fee required for the transaction size.
	feePerByte := estimateFeePerByte(state.lib)
	sizeBytes := sizeOfP2PKH(len(state.inputs), len(state.outputs)+1) // +1 for Change output
	fee := feeForTxn(sizeBytes, feePerByte, fixedFee, maxFee)
	// Deduct the fee from all outputs as per DeductFeePercent (update state.outputs)
	for i, pay := range payTo {
		err := subtractFeeFromOutput(i, fee, pay.DeductFeePercent, state)
		if err != nil {
			return ZeroCoins, err
		}
	}
	return fee, nil
}
