package doge

type Address string // Dogecoin address (base-58 Public Key Hash aka PKH)

// Dogecoin Script Types enum.
// Inferred from ScriptPubKey scripts by pattern-matching the code (script templates)
type ScriptType string

// ScriptType constants - stored in gigawallet database!
const (
	ScriptTypeP2PK     ScriptType = "p2pk"     // TX_PUBKEY (in Core)
	ScriptTypeP2PKH    ScriptType = "p2pkh"    // TX_PUBKEYHASH
	ScriptTypeP2PKHW   ScriptType = "p2wpkh"   // TX_WITNESS_V0_KEYHASH
	ScriptTypeP2SH     ScriptType = "p2sh"     // TX_SCRIPTHASH
	ScriptTypeP2SHW    ScriptType = "p2wsh"    // TX_WITNESS_V0_SCRIPTHASH
	ScriptTypeMultiSig ScriptType = "multisig" // TX_MULTISIG
	ScriptTypeNullData ScriptType = "nulldata" // TX_NULL_DATA
	ScriptTypeCustom   ScriptType = "custom"   // TX_NONSTANDARD
)

func ClassifyScript(script []byte, chain *ChainParams) (ScriptType, Address) {
	L := len(script)
	// P2PKH: OP_DUP OP_HASH160 <pubKeyHash:20> OP_EQUALVERIFY OP_CHECKSIG (25)
	if L == 25 && script[0] == OP_DUP && script[1] == OP_HASH160 && script[2] == 20 &&
		script[23] == OP_EQUALVERIFY && script[24] == OP_CHECKSIG {
		addr := Hash160toAddress(script[3:23], chain.p2pkh_address_prefix)
		return ScriptTypeP2PKH, addr
	}
	// P2PK: <compressedPubKey:33> OP_CHECKSIG
	if L == 35 && script[0] == 33 && script[34] == OP_CHECKSIG {
		// no Base58 Address for P2PK.
		return ScriptTypeP2PK, ""
	}
	// P2PK: <uncompressedPubKey:65> OP_CHECKSIG
	if L == 67 && script[0] == 65 && script[66] == OP_CHECKSIG {
		// no Base58 Address for P2PK.
		return ScriptTypeP2PK, ""
	}
	// P2SH: OP_HASH160 0x14 <hash> OP_EQUAL
	if L == 23 && script[0] == OP_HASH160 && script[1] == 20 && script[22] == OP_EQUAL {
		addr := Hash160toAddress(script[2:22], chain.p2sh_address_prefix)
		return ScriptTypeP2SH, addr
	}
	// OP_m <pubkey*n> OP_n OP_CHECKMULTISIG
	if L >= 3+34 && script[L-1] == OP_CHECKMULTISIG && isOpN1(script[L-2]) && isOpN1(script[0]) {
		numKeys := script[L-2] - (OP_1 - 1)
		endKeys := L - 2
		ofs := 1
		for ofs < endKeys && numKeys > 0 {
			if script[ofs] == 65 && ofs+66 <= endKeys {
				// no Base58 Address for PubKey.
				ofs += 66
			} else if script[ofs] == 33 && ofs+34 <= endKeys {
				// no Base58 Address for PubKey.
				ofs += 34
			} else {
				break
			}
			numKeys -= 1
		}
		if ofs == endKeys && numKeys == 0 { // valid.
			return ScriptTypeMultiSig, ""
		}
		return ScriptTypeCustom, ""
	}
	// OP_RETURN
	if L > 0 && script[0] == OP_RETURN {
		return ScriptTypeNullData, ""
	}
	return ScriptTypeCustom, ""
}

func isOpN1(op byte) bool {
	return op >= OP_1 && op <= OP_16
}
