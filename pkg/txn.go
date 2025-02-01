package giga

import (
	"encoding/hex"
	"log"

	"github.com/dogecoinfoundation/gigawallet/pkg/doge"
	"github.com/shopspring/decimal"
)

var oneHundred = decimal.NewFromInt(100)
var oneThousand = decimal.NewFromInt(1000)

// We may need to use addUTXOsUpToAmount during calculateFee (source, inputs, used)
// and we need to generate the tx hex repeatedly (lib, acc, inputs, outputs, changeAddress)
type txState struct {
	lib       L1         // L1 for MakeTransaction
	account   Account    // Account for private key to sign transactions
	inputs    []UTXO     // accumulated tx inputs (from addUTXOsUpToAmount)
	outputs   []NewTxOut // specified tx outputs (from payTo)
	outputSum CoinAmount // sum specified tx outputs (from payTo)
	used      UTXOSet    // accumulated tx inputs (as a set)
	source    UTXOSource // source of available UTXOs (cached on Account)
	deductFee bool       // payTo has DeductFeePercent specified
}

func CreateTxn(payTo []PayTo, fixedFee CoinAmount, maxFee CoinAmount, acc Account, source UTXOSource, lib L1) (newTx NewTxn, change UTXO, inputs []UTXO, txid string, err error) {
	outputSum, deductFee, err := sumPayTo(payTo)
	if err != nil {
		return
	}
	changeAddress, changeIndex, err := acc.NextChangeAddress(lib)
	if err != nil {
		return
	}
	state := &txState{
		lib:       lib,
		account:   acc,
		inputs:    []UTXO{},
		outputs:   []NewTxOut{},
		outputSum: outputSum,
		used:      NewUTXOSet(),
		source:    source,
		deductFee: deductFee,
	}

	err = addUTXOsUpToAmount(outputSum, state)
	if err != nil {
		return
	}
	for _, pay := range payTo {
		err = addOutput(pay.PayTo, pay.Amount, state)
		if err != nil {
			return
		}
	}

	var fee CoinAmount
	if deductFee {
		fee, err = calculateAndDeductFee(fixedFee, maxFee, payTo, state)
		if err != nil {
			return
		}
	} else {
		fee, err = calculateFee(fixedFee, maxFee, state)
		if err != nil {
			return
		}
	}

	// Build the transaction with the current inputs and fee.
	newTx, err = state.lib.MakeTransaction(state.inputs, state.outputs, fee, changeAddress, state.account.Privkey)
	if err != nil {
		return
	}

	// Check all outputs are >= TxnDustLimit
	txData, err := doge.HexDecode(newTx.TxnHex)
	if err != nil {
		return
	}
	txid = doge.TxHashHex(txData)
	dTx := doge.DecodeTx(txData)
	chain := doge.ChainFromWIFString(string(acc.Address))
	for n, out := range dTx.VOut {
		// This should be caught before now, e.g. in subtractFeeFromOutput.
		stype, addr := doge.ClassifyScript(out.Script, chain)
		if stype == doge.ScriptTypeP2PKH && addr == changeAddress {
			if out.Value < TxnDustLimit_64 {
				err = NewErr(InvalidTxn, "BUG: Tx Change Output cannot be less than the Dogecoin Dust Limit (%vƉ): tx output %v is %v koinu", TxnDustLimit.String(), n, doge.KoinuToDecimal(out.Value).String())
				return
			}
			change = UTXO{
				TxID:          txid,
				VOut:          n,
				Value:         doge.KoinuToDecimal(out.Value),
				ScriptHex:     hex.EncodeToString(out.Script),
				ScriptType:    stype,
				ScriptAddress: addr,
				AccountID:     acc.Address,
				KeyIndex:      changeIndex,
				IsInternal:    true,
			}
		} else if out.Value < TxnDustLimit_64 {
			err = NewErr(InvalidTxn, "BUG: Tx Output cannot be less than the Dogecoin Dust Limit (%vƉ): tx output %v is %v koinu", TxnDustLimit.String(), n, doge.KoinuToDecimal(out.Value).String())
			return
		}
	}

	return newTx, change, state.inputs, txid, nil
}

func sumPayTo(payTo []PayTo) (CoinAmount, bool, error) {
	total := decimal.Zero
	deduct := decimal.Zero
	for _, pay := range payTo {
		// Round the parsed decimal towards -infinity so a split payment using
		// floating point doesn't round to a sum greater than the account balance.
		// This only applies if more than 8 Koinu digits are provided.
		pay.Amount = pay.Amount.RoundFloor(NumKoinuDigits) // round to whole Koinu
		total = total.Add(pay.Amount)
		if pay.Amount.LessThan(TxnDustLimit) {
			return ZeroCoins, false, NewErr(InvalidTxn, "PayTo Amount cannot be less than the Dogecoin Dust Limit (%vƉ): The request was to pay %vƉ to %v", TxnDustLimit, pay.Amount, pay.PayTo)
		}
		if pay.DeductFeePercent.IsNegative() {
			return ZeroCoins, false, NewErr(InvalidTxn, "Deduct-Fee-Percent cannot be negative: The request was to deduct %v%% from a payment of %vƉ to %v", pay.DeductFeePercent, pay.Amount, pay.PayTo)
		} else {
			deduct = deduct.Add(pay.DeductFeePercent)
		}
	}
	deductFee := !deduct.IsZero()
	if deductFee && !deduct.Equals(oneHundred) {
		return ZeroCoins, false, NewErr(InvalidTxn, "The requested Deduct-Fee-Percent values do not add up to 100")
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
	if payTo == "" {
		return NewErr(InvalidTxn, "Invalid transaction output: missing 'to' address in the request.")
	}
	if amount.LessThanOrEqual(ZeroCoins) {
		return NewErr(InvalidTxn, "Invalid transaction output: the 'amount' is negative or zero.")
	}
	state.outputs = append(state.outputs, NewTxOut{
		ScriptType:    doge.ScriptTypeP2PKH,
		Amount:        amount,
		ScriptAddress: payTo,
	})
	return nil
}

// Adjust an output's amount by deducting a percentage of the fee.
// This uses an array of originalAmounts captured by addOutput.
func subtractFeeFromOutput(output int, fee decimal.Decimal, feePercent decimal.Decimal, state *txState) (CoinAmount, error) {
	// Round the fee to a whole number of Koinu (so the Output is also rounded)
	// any rounded-off excess goes to fee because it's not included in any output.
	feeAmount := fee.Mul(feePercent.Div(oneHundred)).Round(NumKoinuDigits)
	if feeAmount.IsPositive() {
		outAmt := state.outputs[output].Amount
		newAmount := outAmt.Sub(feeAmount)
		if newAmount.LessThan(TxnDustLimit) {
			// TODO: we may need to offer the option to drop outputs below the dust limit,
			// i.e. skip the payment to a party whose fee contribution consumes the entire payment.
			return ZeroCoins, NewErr(InvalidTxn,
				"After subtracting the fee percentage, the PayTo Amount is less than the Dogecoin Dust Limit (%vƉ): The request was to pay %vƉ to %v subtracting %v%% of the fee %vƉ which leaves %vƉ remaining.",
				TxnDustLimit, outAmt, state.outputs[output].ScriptAddress,
				feePercent, fee, newAmount)
		}
		state.outputs[output].Amount = newAmount
	}
	return feeAmount, nil
}

// Calculate the size of a P2PKH transaction.
func sizeOfP2PKH(nIn int, nOut int) int64 {
	// inspired by https://bitcoinops.org/en/tools/calc-size/
	// max size for up to 252 inputs and outputs
	return 10 + int64(nIn)*148 + int64(nOut)*34
}

// Get fee estimate from Core `estimatesmartfee` if available,
// otherwise use the base consensus TxnFeePerByte.
func estimateFeePerByte(lib L1) CoinAmount {
	feePerKB, err := lib.EstimateFee(6)
	if err != nil {
		log.Printf("feeForP2PKH: did not use estimatesmartfee due to error: %s", err.Error())
	} else {
		perByte := feePerKB.Div(oneThousand)
		return decimal.Max(perByte, TxnFeePerByte)
	}
	return TxnFeePerByte
}

// Calculate fee based on transacion size, within fee limits.
func feeForTxn(sizeBytes int64, feePerByte CoinAmount, fixedFee CoinAmount, maxFee CoinAmount) CoinAmount {
	if fixedFee.IsPositive() {
		return fixedFee.Round(NumKoinuDigits) // override with specified fee
	}
	feeForSize := feePerByte.Mul(decimal.NewFromInt(sizeBytes))
	feeForSize = decimal.Max(feeForSize, TxnRecommendedMinFee)   // at least minFee
	return decimal.Min(feeForSize, maxFee).Round(NumKoinuDigits) // limit to maxFee, round to Koinu
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
	deductedFee := ZeroCoins
	for i, pay := range payTo {
		amt, err := subtractFeeFromOutput(i, fee, pay.DeductFeePercent, state)
		if err != nil {
			return ZeroCoins, err
		}
		deductedFee = deductedFee.Add(amt)
	}
	return deductedFee, nil
}
