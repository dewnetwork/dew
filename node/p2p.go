package node

import (
	"github.com/dewnetwork/dew/core/types"
)

// P2PBackend adapts Node for p2p.ChainBackend and p2p.TxBackend.
type P2PBackend struct {
	N *Node
}

// Height returns the current chain tip height.
func (b *P2PBackend) Height() uint64 {
	return b.N.BlockNumber()
}

// HeadHash returns the hash of the current head block.
func (b *P2PBackend) HeadHash() types.Hash {
	return b.N.CurrentHeader().Hash()
}

// BlockByNumber returns the encoded block at height n.
func (b *P2PBackend) BlockByNumber(n uint64) (raw []byte, hash types.Hash, ok bool) {
	blk := b.N.GetBlockByNumber(n)
	if blk == nil {
		return nil, types.Hash{}, false
	}
	raw, err := blk.MarshalBinary()
	if err != nil {
		return nil, types.Hash{}, false
	}
	return raw, blk.Hash(), true
}

// BlockByHash returns the encoded block with the given hash.
func (b *P2PBackend) BlockByHash(h types.Hash) (raw []byte, number uint64, ok bool) {
	blk := b.N.GetBlockByHash(h)
	if blk == nil {
		return nil, 0, false
	}
	raw, err := blk.MarshalBinary()
	if err != nil {
		return nil, 0, false
	}
	return raw, blk.Number(), true
}

// HasBlock reports whether the block hash is known.
func (b *P2PBackend) HasBlock(h types.Hash) bool {
	return b.N.GetBlockByHash(h) != nil
}

// HasTx reports whether a pending mempool tx is known.
func (b *P2PBackend) HasTx(h types.Hash) bool {
	return b.N.Mempool().Get(h) != nil
}

// GetTx returns the raw bytes of a pending mempool tx.
func (b *P2PBackend) GetTx(h types.Hash) (raw []byte, ok bool) {
	e := b.N.Mempool().Get(h)
	if e == nil {
		return nil, false
	}
	return append([]byte(nil), e.Raw...), true
}