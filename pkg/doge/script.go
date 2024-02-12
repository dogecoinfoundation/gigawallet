package doge

import "github.com/decred/dcrd/dcrec/secp256k1/v4"

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

func ClassifyScript(script []byte, chain *ChainParams) (ScriptType, []Address) {
	var addr []Address
	L := len(script)
	// P2PKH: OP_DUP OP_HASH160 <pubKeyHash:20> OP_EQUALVERIFY OP_CHECKSIG (25)
	if L == 25 && script[0] == OP_DUP && script[1] == OP_HASH160 && script[2] == 20 &&
		script[23] == OP_EQUALVERIFY && script[24] == OP_CHECKSIG {
		addr = append(addr, Hash160toAddress(script[3:23], chain.p2pkh_address_prefix))
		return ScriptTypeP2PKH, addr
	}
	// <compressedPubKey:33> OP_CHECKSIG
	if L == 35 && script[0] == 33 && (script[1] == 0x02 || script[1] == 0x03) && script[34] == OP_CHECKSIG {
		addr = append(addr, PubKeyToAddress(script[1:34], chain))
		return ScriptTypeP2PK, addr
	}
	// <uncompressedPubKey:65> OP_CHECKSIG
	if L == 67 && script[0] == 65 && script[1] == 0x04 && script[66] == OP_CHECKSIG {
		key, err := secp256k1.ParsePubKey(script[1:66])
		if err == nil {
			addr = append(addr, PubKeyToAddress(key.SerializeCompressed(), chain))
			return ScriptTypeP2PK, addr
		}
	}
	// OP_HASH160 0x14 <hash> OP_EQUAL
	if L == 23 && script[0] == OP_HASH160 && script[1] == 20 && script[22] == OP_EQUAL {
		addr = append(addr, Hash160toAddress(script[2:22], chain.p2sh_address_prefix))
		return ScriptTypeP2SH, addr
	}
	// OP_m <pubkey*n> OP_n OP_CHECKMULTISIG
	if L > 4 && script[L-1] == OP_CHECKMULTISIG && isOpN(script[L-2]) && isOpN(script[0]) {
		// TODO: decode addresses (not necessary for current Gigawallet use-case)
		return ScriptTypeMultiSig, addr
	}
	// OP_RETURN
	if L > 0 && script[0] == OP_RETURN {
		return ScriptTypeNullData, nil
	}
	return ScriptTypeCustom, nil
}

func isOpN(op byte) bool {
	return (op >= OP_1 && op <= OP_16) || op == OP_0
}

func isPushOp(op byte) bool {
	return op <= OP_PUSHDATA4
}

// Return the push-data length in the 1st opcode of script.
func decodePushLen(script []byte) (length uint64, used uint64) {
	op := script[0]
	if op <= 0x4b {
		return uint64(op), 1
	}
	switch op {
	case OP_PUSHDATA1:
		return uint64(script[1]), 2
	case OP_PUSHDATA2:
		return uint64(script[1])<<8 | uint64(script[2]), 3
	case OP_PUSHDATA4:
	}
	return 0, 0
}
