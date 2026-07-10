package types

import (
	"math/big"

	"github.com/dewnetwork/dew/crypto"
)

// Block is a header plus body (transactions).
type Block struct {
	header       *Header
	transactions []*Transaction
}

// NewBlock creates a block. TxRoot is derived from transactions unless header already sets a non-empty body root intentionally.
func NewBlock(header *Header, txs []*Transaction) *Block {
	h := header.Copy()
	if h.TxRoot.IsZero() {
		h.TxRoot = TxRoot(txs)
	}
	if h.ReceiptRoot.IsZero() {
		h.ReceiptRoot = EmptyReceiptRoot
	}
	body := make([]*Transaction, len(txs))
	copy(body, txs)
	return &Block{header: h, transactions: body}
}

// NewGenesisBlock builds height-0 block with empty txs.
func NewGenesisBlock(header *Header) *Block {
	h := header.Copy()
	h.Number = 0
	h.ParentHash = Hash{}
	h.TxRoot = EmptyTxRoot
	h.ReceiptRoot = EmptyReceiptRoot
	h.GasUsed = 0
	if h.BaseFee == nil {
		h.BaseFee = big.NewInt(0)
	}
	return &Block{header: h, transactions: nil}
}

// Header returns a copy of the block header.
func (b *Block) Header() *Header {
	return b.header.Copy()
}

// Hash returns the header hash.
func (b *Block) Hash() Hash {
	return b.header.Hash()
}

// Number returns the block number.
func (b *Block) Number() uint64 {
	return b.header.Number
}

// Transactions returns the transaction list (not a deep copy of each tx).
func (b *Block) Transactions() []*Transaction {
	out := make([]*Transaction, len(b.transactions))
	copy(out, b.transactions)
	return out
}

// WithStateRoot returns a new block with updated state root (immutability helper).
func (b *Block) WithStateRoot(root Hash) *Block {
	h := b.header.Copy()
	h.StateRoot = root
	return &Block{header: h, transactions: b.transactions}
}

// EmptyProposer is the zero address used when no proposer is set (e.g. genesis).
var EmptyProposer crypto.Address
