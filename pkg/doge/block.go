package doge

import (
	"fmt"
)

const (
	VersionAuxPoW = 256
	CoinbaseVOut  = 0xffffffff
	MaxScriptSize = 10_000 // MAX_SCRIPT_SIZE from Dogecoin Core
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

type MerkleTx struct {
	CoinbaseTx       BlockTx
	ParentHash       []byte // 32 bytes
	CoinbaseBranch   MerkleBranch
	BlockchainBranch MerkleBranch
	ParentBlock      BlockHeader
}

type MerkleBranch struct {
	Hash     [][]byte // 32 bytes each
	SideMask uint32
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
}

type BlockTxOut struct {
	Value  int64
	Script []byte // varied length
}

func DecodeBlock(blockBytes []byte) (Block, error) {
	s := &Stream{b: blockBytes}
	return readBlock(s)
}

func readBlock(s *Stream) (b Block, err error) {
	b.Header = readHeader(s)
	if b.Header.IsAuxPoW() {
		auxPow, err := readMerkleTx(s)
		if err != nil {
			return b, fmt.Errorf("error reading AuxPoW: %v", err)
		}
		b.AuxPoW = auxPow
	}
	numTx := s.var_uint()
	for i := uint64(0); i < numTx; i++ {
		tx, err := readTx(s)
		if err != nil {
			return b, fmt.Errorf("error reading transaction %d: %v", i, err)
		}
		b.Tx = append(b.Tx, tx)
	}
	return b, nil
}

func readHeader(s *Stream) (b BlockHeader) {
	b.Version = s.uint32le()
	b.PrevBlock = s.bytes(32)
	b.MerkleRoot = s.bytes(32)
	b.Timestamp = s.uint32le()
	b.Bits = s.uint32le()
	b.Nonce = s.uint32le()
	return
}

func readMerkleTx(s *Stream) (*MerkleTx, error) {
	var m MerkleTx
	coinbaseTx, err := readTx(s)
	if err != nil {
		return nil, fmt.Errorf("error reading coinbase tx: %v", err)
	}
	m.CoinbaseTx = coinbaseTx
	m.ParentHash = s.bytes(32)
	m.CoinbaseBranch = readMerkleBranch(s)
	m.BlockchainBranch = readMerkleBranch(s)
	m.ParentBlock = readHeader(s)
	return &m, nil
}

func readMerkleBranch(s *Stream) (b MerkleBranch) {
	numHash := s.var_uint()
	for i := uint64(0); i < numHash; i++ {
		b.Hash = append(b.Hash, s.bytes(32))
	}
	b.SideMask = s.uint32le()
	return
}

func DecodeTx(txBytes []byte) (BlockTx, error) {
	s := &Stream{b: txBytes}
	return readTx(s)
}

func readTx(s *Stream) (tx BlockTx, err error) {
	start := s.p
	tx.Version = s.uint32le()
	tx_in := s.var_uint()
	for i := uint64(0); i < tx_in; i++ {
		vin, err := readTxIn(s)
		if err != nil {
			return tx, fmt.Errorf("error reading tx input %d: %v", i, err)
		}
		tx.VIn = append(tx.VIn, vin)
	}
	tx_out := s.var_uint()
	for i := uint64(0); i < tx_out; i++ {
		vout, err := readTxOut(s)
		if err != nil {
			return tx, fmt.Errorf("error reading tx output %d: %v", i, err)
		}
		tx.VOut = append(tx.VOut, vout)
	}
	tx.LockTime = s.uint32le()
	// Compute TX hash from transaction bytes.
	tx.TxID = TxHashHex(s.b[start:s.p])
	return tx, nil
}

func readTxIn(s *Stream) (BlockTxIn, error) {
	var in BlockTxIn
	in.TxID = s.bytes(32)
	in.VOut = s.uint32le()
	scriptLen := s.var_uint()
	if scriptLen > MaxScriptSize {
		return in, fmt.Errorf("script length %d exceeds maximum allowed size of %d", scriptLen, MaxScriptSize)
	}
	in.Script = s.bytes(uint64(scriptLen))
	in.Sequence = s.uint32le()
	return in, nil
}

func readTxOut(s *Stream) (BlockTxOut, error) {
	var out BlockTxOut
	out.Value = int64(s.uint64le())
	scriptLen := s.var_uint()
	if scriptLen > MaxScriptSize {
		return out, fmt.Errorf("script length %d exceeds maximum allowed size of %d", scriptLen, MaxScriptSize)
	}
	out.Script = s.bytes(uint64(scriptLen))
	return out, nil
}
