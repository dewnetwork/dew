package p2p

import (
	"sync"

	"github.com/dewnetwork/dew/core/types"
)

// ChainBackend supplies chain tip and historical blocks for handshake, gossip, and sync.
type ChainBackend interface {
	Height() uint64
	HeadHash() types.Hash
	// BlockByNumber returns encoded block payload and hash at height (false if missing).
	BlockByNumber(n uint64) (raw []byte, hash types.Hash, ok bool)
	// BlockByHash returns encoded block payload (false if missing).
	BlockByHash(h types.Hash) (raw []byte, number uint64, ok bool)
	// HasBlock reports whether the block is known.
	HasBlock(h types.Hash) bool
}

// TxBackend supplies mempool / recent txs for gossip.
type TxBackend interface {
	HasTx(h types.Hash) bool
	GetTx(h types.Hash) (raw []byte, ok bool)
}

// AppHandlers receive validated network objects (Host does not re-validate signatures
// beyond handshake — app layer should).
type AppHandlers struct {
	// OnTx is called when a full tx payload arrives. Return nil to accept (and allow re-gossip).
	OnTx func(hash types.Hash, raw []byte, from PeerID) error
	// OnBlock is called when a full block payload arrives.
	OnBlock func(number uint64, hash types.Hash, raw []byte, from PeerID) error
	// OnProposal delivers a consensus proposal.
	OnProposal func(msg *WireProposal, from PeerID) error
	// OnVote delivers a consensus prevote/precommit.
	OnVote func(msg *WireVote, from PeerID) error
}

// MemoryChain is an in-memory ChainBackend + TxBackend for tests and local nets.
type MemoryChain struct {
	mu     sync.RWMutex
	height uint64
	// number -> payload
	blocks map[uint64]blockRec
	byHash map[types.Hash]uint64
	txs    map[types.Hash][]byte
}

type blockRec struct {
	hash types.Hash
	raw  []byte
}

// NewMemoryChain starts at height 0 with an optional genesis payload.
func NewMemoryChain(genesisHash types.Hash, genesisRaw []byte) *MemoryChain {
	mc := &MemoryChain{
		height: 0,
		blocks: make(map[uint64]blockRec),
		byHash: make(map[types.Hash]uint64),
		txs:    make(map[types.Hash][]byte),
	}
	if !genesisHash.IsZero() || len(genesisRaw) > 0 {
		mc.blocks[0] = blockRec{hash: genesisHash, raw: genesisRaw}
		mc.byHash[genesisHash] = 0
	}
	return mc
}

// Height implements ChainBackend.
func (m *MemoryChain) Height() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.height
}

// HeadHash implements ChainBackend.
func (m *MemoryChain) HeadHash() types.Hash {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.blocks[m.height]
	if !ok {
		return types.Hash{}
	}
	return rec.hash
}

// BlockByNumber implements ChainBackend.
func (m *MemoryChain) BlockByNumber(n uint64) ([]byte, types.Hash, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.blocks[n]
	if !ok {
		return nil, types.Hash{}, false
	}
	return append([]byte(nil), rec.raw...), rec.hash, true
}

// BlockByHash implements ChainBackend.
func (m *MemoryChain) BlockByHash(h types.Hash) ([]byte, uint64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.byHash[h]
	if !ok {
		return nil, 0, false
	}
	rec := m.blocks[n]
	return append([]byte(nil), rec.raw...), n, true
}

// HasBlock implements ChainBackend.
func (m *MemoryChain) HasBlock(h types.Hash) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.byHash[h]
	return ok
}

// AddBlock appends a block at number (must be height+1, or 0 for genesis replace).
func (m *MemoryChain) AddBlock(number uint64, hash types.Hash, raw []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[number] = blockRec{hash: hash, raw: append([]byte(nil), raw...)}
	m.byHash[hash] = number
	if number > m.height {
		m.height = number
	}
	if number == 0 && m.height == 0 {
		m.height = 0
	}
}

// AddTx stores a tx for gossip serving.
func (m *MemoryChain) AddTx(hash types.Hash, raw []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs[hash] = append([]byte(nil), raw...)
}

// HasTx implements TxBackend.
func (m *MemoryChain) HasTx(h types.Hash) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.txs[h]
	return ok
}

// GetTx implements TxBackend.
func (m *MemoryChain) GetTx(h types.Hash) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	raw, ok := m.txs[h]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), raw...), true
}
