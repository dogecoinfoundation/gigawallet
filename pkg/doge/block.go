package doge

import (
	"fmt"
	"log"
)

const (
	VersionAuxPoW = 256
	CoinbaseVOut  = 0xffffffff
	MaxScriptSize = 10_000     // MAX_SCRIPT_SIZE from Dogecoin Core (script.h)
	MaxVarIntSize = 0x02000000 // MAX_SIZE from Dogecoin Core (serialize.h)
)

var CoinbaseTxID = [32]byte{}

type Block struct {
	Header BlockHeader
	AuxPoW *MerkleTx // if IsAuxPoW()
	Tx     []BlockTx
}

type BlockHeader struct {
	Version    uint32
	PrevBlock  []byte // 32 bytes
	MerkleRoot []byte // 32 bytes
	Timestamp  uint32
	Bits       uint32
	Nonce      uint32
}

func (b *BlockHeader) IsAuxPoW() bool {
	return (b.Version & VersionAuxPoW) != 0
}

// MerkleTx mirrors CMerkleTx in Dogecoin Core.
type MerkleTx struct {
	CoinbaseTx       BlockTx      // CMerkleTx:: `tx` in Core: CTransaction
	ParentHash       []byte       // CMerkleTx:: `hashBlock` in Core: uint256 (32-byte hash)
	CoinbaseBranch   MerkleBranch // CMerkleTx:: `vMerkleBranch` and `nIndex` in Core
	BlockchainBranch MerkleBranch // CAuxPow:: `vChainMerkleBranch` and `nChainIndex` in Core
	ParentBlock      BlockHeader  // CAuxPow:: `parentBlock` in Core: CPureBlockHeader
}

type MerkleBranch struct {
	Hash     [][]byte // `vMerkleBranch` in Core: vector of uint256 (32-byte hashes)
	SideMask uint32   // `nIndex` in Core: uint32le
}

type BlockTx struct {
	Version  uint32
	VIn      []BlockTxIn
	VOut     []BlockTxOut
	LockTime uint32
	TxID     string // hex, computed from tx data
}

type BlockTxIn struct {
	TxID     []byte // 32 bytes
	VOut     uint32
	Script   []byte // varied length
	Sequence uint32
	Witness  [][]byte // optional witness data
}

type BlockTxOut struct {
	Value  int64
	Script []byte // varied length
}

func DecodeBlock(blockBytes []byte, blockid string) (b Block, err error) {
	s := NewStream(blockBytes)
	b, err = readBlock(s, blockid)
	if err == nil && !s.Complete() {
		if s.Valid() {
			err = fmt.Errorf("error reading block: did not use all data: %v of %v", s.pos, s.len)
		} else {
			err = fmt.Errorf("error reading block: overran end of data: %v of %v", s.pos, s.len)
		}
	}
	return
}

func readBlock(s *Stream, blockid string) (b Block, err error) {
	b.Header = readHeader(s)
	if b.Header.IsAuxPoW() {
		auxPow, err := readMerkleTx(s, "AuxPoW "+blockid)
		if err != nil {
			return b, fmt.Errorf("error reading AuxPoW: %v", err)
		}
		b.AuxPoW = auxPow
	}
	blkid := "block " + blockid
	numTx := s.VarUint()
	for i := uint64(0); i < numTx; i++ {
		tx, err := readTx(s, blkid)
		if err != nil {
			return b, fmt.Errorf("error reading transaction %d: %v", i, err)
		}
		b.Tx = append(b.Tx, tx)
	}
	return b, nil
}

func readHeader(s *Stream) (b BlockHeader) {
	b.Version = s.Uint32le()
	b.PrevBlock = s.Bytes(32)
	b.MerkleRoot = s.Bytes(32)
	b.Timestamp = s.Uint32le()
	b.Bits = s.Uint32le()
	b.Nonce = s.Uint32le()
	return
}

func readMerkleTx(s *Stream, blockid string) (*MerkleTx, error) {
	var m MerkleTx
	coinbaseTx, err := readTx(s, blockid)
	if err != nil {
		return nil, fmt.Errorf("error reading coinbase tx: %v", err)
	}
	m.CoinbaseTx = coinbaseTx
	m.ParentHash = s.Bytes(32)
	m.CoinbaseBranch = readMerkleBranch(s)
	m.BlockchainBranch = readMerkleBranch(s)
	m.ParentBlock = readHeader(s)
	return &m, nil
}

func readMerkleBranch(s *Stream) (b MerkleBranch) {
	numHash := s.VarUint()
	for i := uint64(0); i < numHash; i++ {
		b.Hash = append(b.Hash, s.Bytes(32))
	}
	b.SideMask = s.Uint32le()
	return
}

func DecodeTx(txBytes []byte, txid string) (BlockTx, error) {
	s := NewStream(txBytes)
	return readTx(s, txid)
}

func readTx(s *Stream, txid string) (tx BlockTx, err error) {
	start := s.pos
	flags := uint8(0)
	tx.Version = s.Uint32le()
	// Detect extended transaction serialization format: "The marker MUST be a 1-byte zero value: 0x00"
	// However, Core uses ReadCompactSize via `s >> tx.vin` in UnserializeTransaction, so anything goes.
	// The following nested if-else structure exactly mirrors Core.
	tx_in := s.VarUint()
	if tx_in == 0 {
		// Extended transaction serialization format.
		log.Printf("[*] NOTE: extended transaction serialization format: %v", txid)
		// "The flag MUST be a 1-byte non-zero value. Currently, 0x01 MUST be used."
		flags = s.Bytes(1)[0]
		if flags != 0 {
			// Here Core parses full VIn and VOut vectors if flags are non-zero.
			tx_in = s.VarUint()
			tx.VIn, tx.VOut, err = readVinVout(s, tx_in)
			if err != nil {
				return tx, err
			}
		} else {
			// VIn/VOut parsing is skipped entirely if flags is zero! (as per Core)
			// This may be an oversight in Core - probably an exploit.
			log.Printf("[!] WARNING: skipped VIn/Vout parsing entirely, as per Core implementation: %v", txid)
		}
	} else {
		// We read a non-empty vin. Assume a normal vout follows (as per Core)
		tx.VIn, tx.VOut, err = readVinVout(s, tx_in)
		if err != nil {
			return tx, err
		}
	}
	// Here Core tests if the low bit is set.
	if (flags & 1) != 0 {
		// The witness flag is present: read witness data.
		// Core parses `std::vector<std::vector<unsigned char>>` per vin.
		flags ^= 1 // Core toggles the low bit.
		for i := uint64(0); i < tx_in; i++ {
			numStackItems := s.VarUint()
			for k := uint64(0); k < numStackItems; k++ {
				itemLen := s.VarUint()
				if itemLen > MaxVarIntSize {
					return tx, fmt.Errorf("witness data too large: %v for vin %v stack item %v", itemLen, i, k)
				}
				itemData := s.Bytes(itemLen)
				tx.VIn[i].Witness = append(tx.VIn[i].Witness, itemData)
			}
		}
	}
	// Here Core treats non-zero flags as an error.
	if flags != 0 {
		return tx, fmt.Errorf("unknown transaction optional data: %v", flags)
	}
	tx.LockTime = s.Uint32le()
	// Compute TX hash from transaction bytes.
	if s.Valid() {
		tx.TxID = TxHashHex(s.buf[start:s.pos])
	}
	return tx, nil
}

func readVinVout(s *Stream, tx_in uint64) (VIn []BlockTxIn, VOut []BlockTxOut, err error) {
	for i := uint64(0); i < tx_in; i++ {
		vin, err := readTxIn(s)
		if err != nil {
			return nil, nil, fmt.Errorf("error reading tx input %d: %v", i, err)
		}
		VIn = append(VIn, vin)
	}
	tx_out := s.VarUint()
	for i := uint64(0); i < tx_out; i++ {
		vout, err := readTxOut(s)
		if err != nil {
			return nil, nil, fmt.Errorf("error reading tx output %d: %v", i, err)
		}
		VOut = append(VOut, vout)
	}
	return
}

func readTxIn(s *Stream) (BlockTxIn, error) {
	var in BlockTxIn
	in.TxID = s.Bytes(32)
	in.VOut = s.Uint32le()
	scriptLen := s.VarUint()
	if scriptLen > MaxScriptSize {
		return in, fmt.Errorf("script length %d exceeds maximum allowed size of %d", scriptLen, MaxScriptSize)
	}
	in.Script = s.Bytes(uint64(scriptLen))
	in.Sequence = s.Uint32le()
	return in, nil
}

func readTxOut(s *Stream) (BlockTxOut, error) {
	var out BlockTxOut
	out.Value = int64(s.Uint64le())
	scriptLen := s.VarUint()
	if scriptLen > MaxScriptSize {
		return out, fmt.Errorf("script length %d exceeds maximum allowed size of %d", scriptLen, MaxScriptSize)
	}
	out.Script = s.Bytes(uint64(scriptLen))
	return out, nil
}
