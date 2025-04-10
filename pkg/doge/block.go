package doge

import "log"

const (
	VersionAuxPoW = 256
	CoinbaseVOut  = 0xffffffff
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
	Hash       string // hex, computed from block data
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

func DecodeBlock(blockBytes []byte) Block {
	s := &Stream{b: blockBytes}
	return readBlock(s)
}

func readBlock(s *Stream) (b Block) {
	b.Header = readHeader(s)
	if b.Header.IsAuxPoW() {
		b.AuxPoW = readMerkleTx(s)
	}
	numTx := s.var_uint()
	for i := uint64(0); i < numTx; i++ {
		b.Tx = append(b.Tx, readTx(s))
	}
	return
}

func readHeader(s *Stream) (b BlockHeader) {
	start := s.p
	b.Version = s.uint32le()
	b.PrevBlock = s.bytes(32)
	b.MerkleRoot = s.bytes(32)
	b.Timestamp = s.uint32le()
	b.Bits = s.uint32le()
	b.Nonce = s.uint32le()
	// (version, prev_block, merkle_root, timestamp, bits, nonce)
	b.Hash = BlockHashHex(s.b[start:s.p])
	return
}

func readMerkleTx(s *Stream) *MerkleTx {
	var m MerkleTx
	m.CoinbaseTx = readTx(s)
	m.ParentHash = s.bytes(32)
	m.CoinbaseBranch = readMerkleBranch(s)
	m.BlockchainBranch = readMerkleBranch(s)
	m.ParentBlock = readHeader(s)
	return &m
}

func readMerkleBranch(s *Stream) (b MerkleBranch) {
	numHash := s.var_uint()
	for i := uint64(0); i < numHash; i++ {
		b.Hash = append(b.Hash, s.bytes(32))
	}
	b.SideMask = s.uint32le()
	return
}

func DecodeTx(txBytes []byte) BlockTx {
	s := &Stream{b: txBytes}
	return readTx(s)
}

func readTx(s *Stream) (tx BlockTx) {
	log.Println("tx.s.p", len(s.b), s.p)
	start := s.p
	tx.Version = s.uint32le()
	log.Printf("tx.Version: %v", tx.Version)
	tx_in := s.var_uint()
	log.Println("tx.tx_in", tx_in)
	for i := uint64(0); i < tx_in; i++ {
		tx.VIn = append(tx.VIn, readTxIn(s))
	}
	tx_out := s.var_uint()
	log.Println("tx.tx_out", tx_out)
	for i := uint64(0); i < tx_out; i++ {
		tx.VOut = append(tx.VOut, readTxOut(s))
	}
	tx.LockTime = s.uint32le()
	log.Printf("tx.LockTime: %v", tx.LockTime)
	// Compute TX hash from transaction bytes.
	tx.TxID = TxHashHex(s.b[start:s.p])
	log.Printf("tx.TxID: %v", tx.TxID)
	return
}

func readTxIn(s *Stream) (in BlockTxIn) {
	log.Println("in.s.p", len(s.b), s.p)
	in.TxID = s.bytes(32)
	log.Printf("in.TxID: %v", in.TxID)
	in.VOut = s.uint32le()
	log.Printf("in.VOut: %v", in.VOut)
	script_len := s.var_uint()
	log.Printf("in.script_len: %v", script_len)
	in.Script = s.bytes(script_len)
	log.Printf("in.Script: %v", in.Script)
	in.Sequence = s.uint32le()
	log.Printf("in.Sequence: %v", in.Sequence)
	return
}

func readTxOut(s *Stream) (out BlockTxOut) {
	log.Println("out.s.p", len(s.b), s.p)
	out.Value = int64(s.uint64le())
	log.Printf("out.Value: %v", out.Value)
	script_len := s.var_uint()
	log.Printf("out.script_len: %v", script_len)
	out.Script = s.bytes(script_len)
	log.Printf("out.Script: %v", out.Script)
	return
}
