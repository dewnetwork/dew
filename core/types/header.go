package types

import (
	"io"
	"math/big"

	"github.com/dewnetwork/dew/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Header is the Dew block header (docs/protocol/blocks.md).
//
// Canonical RLP encoding (order is consensus-critical):
//
//	[ParentHash, StateRoot, TxRoot, ReceiptRoot, Number, Timestamp,
//	 GasLimit, GasUsed, BaseFee, ExtraData, Proposer]
type Header struct {
	ParentHash  Hash
	StateRoot   Hash
	TxRoot      Hash
	ReceiptRoot Hash
	Number      uint64
	Timestamp   uint64
	GasLimit    uint64
	GasUsed     uint64
	BaseFee     *big.Int // EIP-1559; required in Phase A
	ExtraData   []byte   // max 32 bytes (tentative)
	Proposer    crypto.Address
}

// Copy returns a deep copy of the header.
func (h *Header) Copy() *Header {
	if h == nil {
		return nil
	}
	c := *h
	if h.BaseFee != nil {
		c.BaseFee = new(big.Int).Set(h.BaseFee)
	}
	if h.ExtraData != nil {
		c.ExtraData = append([]byte(nil), h.ExtraData...)
	}
	return &c
}

// EncodeRLP implements rlp.Encoder for the canonical header list.
func (h *Header) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		h.ParentHash.Bytes(),
		h.StateRoot.Bytes(),
		h.TxRoot.Bytes(),
		h.ReceiptRoot.Bytes(),
		h.Number,
		h.Timestamp,
		h.GasLimit,
		h.GasUsed,
		h.BaseFee,
		h.ExtraData,
		h.Proposer.Bytes(),
	})
}

// DecodeRLP implements rlp.Decoder.
func (h *Header) DecodeRLP(s *rlp.Stream) error {
	var raw struct {
		ParentHash  []byte
		StateRoot   []byte
		TxRoot      []byte
		ReceiptRoot []byte
		Number      uint64
		Timestamp   uint64
		GasLimit    uint64
		GasUsed     uint64
		BaseFee     *big.Int
		ExtraData   []byte
		Proposer    []byte
	}
	if err := s.Decode(&raw); err != nil {
		return err
	}
	h.ParentHash = BytesToHash(raw.ParentHash)
	h.StateRoot = BytesToHash(raw.StateRoot)
	h.TxRoot = BytesToHash(raw.TxRoot)
	h.ReceiptRoot = BytesToHash(raw.ReceiptRoot)
	h.Number = raw.Number
	h.Timestamp = raw.Timestamp
	h.GasLimit = raw.GasLimit
	h.GasUsed = raw.GasUsed
	h.BaseFee = raw.BaseFee
	h.ExtraData = raw.ExtraData
	if len(raw.Proposer) != 0 && len(raw.Proposer) != 20 {
		return errInvalidProposer
	}
	copy(h.Proposer[:], raw.Proposer)
	return nil
}

var errInvalidProposer = errString("types: proposer must be 20 bytes")

type errString string

func (e errString) Error() string { return string(e) }

// Hash returns Keccak-256 of the canonical RLP encoding.
// This is the block hash used for ParentHash linking.
func (h *Header) Hash() Hash {
	enc, err := rlp.EncodeToBytes(h)
	if err != nil {
		// Encoding only fails on programmer error; panic surfaces it in tests.
		panic("types: header RLP encode: " + err.Error())
	}
	return Keccak256Hash(enc)
}

// EmptyRoots for empty tx/receipt lists (Keccak of RLP empty list).
var (
	// EmptyTxRoot is the TxRoot when the block has no transactions.
	EmptyTxRoot Hash
	// EmptyReceiptRoot is the ReceiptRoot when there are no receipts.
	EmptyReceiptRoot Hash
)

func init() {
	emptyList, err := rlp.EncodeToBytes([]interface{}{})
	if err != nil {
		panic(err)
	}
	EmptyTxRoot = Keccak256Hash(emptyList)
	EmptyReceiptRoot = EmptyTxRoot
}
