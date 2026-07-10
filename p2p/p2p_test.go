package p2p

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func newTestHost(t *testing.T, chain *MemoryChain, handlers AppHandlers) *Host {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if chain == nil {
		chain = NewMemoryChain(types.Hash{}, nil)
	}
	h, err := NewHost(Config{
		PrivateKey: key,
		ChainID:    big.NewInt(2026),
		ListenAddr: "127.0.0.1:0",
		MaxPeers:   10,
	}, chain, chain, handlers)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// TestHandshakeAndPeerStore — A6: handshake + peer store.
func TestHandshakeAndPeerStore(t *testing.T) {
	aChain := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("genesis"))
	bChain := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("genesis"))

	a := newTestHost(t, aChain, AppHandlers{})
	b := newTestHost(t, bChain, AppHandlers{})

	peer, err := a.Dial(b.ListenAddr())
	if err != nil {
		t.Fatal(err)
	}
	if peer.ID.Equal(a.ID()) {
		t.Fatal("peer id is self")
	}
	if !peer.ID.Equal(b.ID()) {
		t.Fatalf("peer id %s want %s", peer.ID.Hex(), b.ID().Hex())
	}

	// Wait until B also sees A
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b.PeerCount() == 1 && a.PeerCount() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if a.PeerCount() != 1 || b.PeerCount() != 1 {
		t.Fatalf("peer counts a=%d b=%d", a.PeerCount(), b.PeerCount())
	}

	// Peer store remembers dialable address
	if _, ok := a.Store().KnownByID(b.ID()); !ok {
		t.Fatal("a does not know b")
	}
	if _, ok := b.Store().KnownByID(a.ID()); !ok {
		t.Fatal("b does not know a")
	}

	// Chain id mismatch disconnects
	badKey, _ := crypto.GenerateKey()
	badChain := NewMemoryChain(types.Hash{}, nil)
	bad, err := NewHost(Config{
		PrivateKey: badKey,
		ChainID:    big.NewInt(999),
		ListenAddr: "127.0.0.1:0",
	}, badChain, badChain, AppHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	if err := bad.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bad.Close() })
	if _, err := bad.Dial(a.ListenAddr()); err == nil {
		t.Fatal("expected chain id mismatch error")
	}
}

// TestTxAndBlockGossip — A6: tx and block inventory gossip.
func TestTxAndBlockGossip(t *testing.T) {
	txHash := types.Keccak256Hash([]byte("tx-1"))
	txRaw := []byte("raw-tx-payload")
	blockHash := types.Keccak256Hash([]byte("block-1"))
	blockRaw := []byte("raw-block-1")

	var mu sync.Mutex
	var gotTx, gotBlock bool

	aChain := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("g"))
	aChain.AddTx(txHash, txRaw)
	aChain.AddBlock(1, blockHash, blockRaw)
	// height becomes 1

	bChain := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("g"))

	a := newTestHost(t, aChain, AppHandlers{})
	b := newTestHost(t, bChain, AppHandlers{
		OnTx: func(hash types.Hash, raw []byte, from PeerID) error {
			mu.Lock()
			defer mu.Unlock()
			if hash == txHash && string(raw) == string(txRaw) {
				gotTx = true
				bChain.AddTx(hash, raw)
			}
			return nil
		},
		OnBlock: func(number uint64, hash types.Hash, raw []byte, from PeerID) error {
			mu.Lock()
			defer mu.Unlock()
			if hash == blockHash && number == 1 {
				gotBlock = true
				bChain.AddBlock(number, hash, raw)
			}
			return nil
		},
	})

	if _, err := a.Dial(b.ListenAddr()); err != nil {
		t.Fatal(err)
	}
	waitPeers(t, a, b)

	if err := a.GossipTx(txHash); err != nil {
		t.Fatal(err)
	}
	if err := a.GossipBlock(blockHash); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := gotTx && gotBlock
		mu.Unlock()
		if ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if !gotTx {
		t.Fatal("tx was not gossiped to b")
	}
	if !gotBlock {
		t.Fatal("block was not gossiped to b")
	}
	if !bChain.HasTx(txHash) || !bChain.HasBlock(blockHash) {
		t.Fatal("b backend missing objects")
	}
}

// TestSyncFromHeightZero — A6: sync from height 0 behind peer.
func TestSyncFromHeightZero(t *testing.T) {
	full := NewMemoryChain(types.Hash{}, nil)
	// heights 0..5
	for i := uint64(0); i <= 5; i++ {
		h := types.Keccak256Hash([]byte{byte(i)})
		full.AddBlock(i, h, []byte{byte(i), 'x'})
	}

	empty := NewMemoryChain(types.Hash{}, nil)

	src := newTestHost(t, full, AppHandlers{})
	dst := newTestHost(t, empty, AppHandlers{
		OnBlock: func(number uint64, hash types.Hash, raw []byte, from PeerID) error {
			empty.AddBlock(number, hash, raw)
			return nil
		},
	})

	peer, err := dst.Dial(src.ListenAddr())
	if err != nil {
		t.Fatal(err)
	}
	waitPeers(t, src, dst)

	// Peer tip from handshake should be 5
	if peer.Height != 5 {
		// handshake captured height; allow brief race
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) && peer.Height != 5 {
			time.Sleep(5 * time.Millisecond)
		}
		if peer.Height != 5 {
			t.Fatalf("peer height %d want 5", peer.Height)
		}
	}

	if err := dst.SyncFromPeer(peer); err != nil {
		t.Fatal(err)
	}
	if empty.Height() != 5 {
		t.Fatalf("synced height %d want 5", empty.Height())
	}
	for i := uint64(0); i <= 5; i++ {
		raw, hash, ok := empty.BlockByNumber(i)
		if !ok {
			t.Fatalf("missing block %d", i)
		}
		wantHash := types.Keccak256Hash([]byte{byte(i)})
		if hash != wantHash || len(raw) == 0 {
			t.Fatalf("block %d mismatch", i)
		}
	}
}

// TestConsensusMessagesDelivered — A6: consensus messages among validators.
func TestConsensusMessagesDelivered(t *testing.T) {
	var mu sync.Mutex
	var gotProp *WireProposal
	var gotVote *WireVote

	chainA := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("g"))
	chainB := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("g"))
	chainC := NewMemoryChain(types.Keccak256Hash([]byte("g")), []byte("g"))

	a := newTestHost(t, chainA, AppHandlers{})
	b := newTestHost(t, chainB, AppHandlers{
		OnProposal: func(msg *WireProposal, from PeerID) error {
			mu.Lock()
			defer mu.Unlock()
			cp := *msg
			gotProp = &cp
			return nil
		},
		OnVote: func(msg *WireVote, from PeerID) error {
			mu.Lock()
			defer mu.Unlock()
			cp := *msg
			gotVote = &cp
			return nil
		},
	})
	c := newTestHost(t, chainC, AppHandlers{
		OnProposal: func(msg *WireProposal, from PeerID) error {
			return nil
		},
		OnVote: func(msg *WireVote, from PeerID) error {
			return nil
		},
	})

	// A — B — C line: A dials B, C dials B; A broadcasts should reach B (and flood to C).
	if _, err := a.Dial(b.ListenAddr()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Dial(b.ListenAddr()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.PeerCount() >= 1 && b.PeerCount() >= 2 && c.PeerCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	prop := &WireProposal{
		Height:    1,
		Round:     0,
		BlockHash: types.Keccak256Hash([]byte("bh")),
		Proposer:  a.ID(),
		Signature: make([]byte, 65),
		BlockRaw:  []byte("blk"),
	}
	if err := a.BroadcastProposal(prop); err != nil {
		t.Fatal(err)
	}
	vote := &WireVote{
		Type:      1,
		Height:    1,
		Round:     0,
		BlockHash: prop.BlockHash,
		Validator: a.ID(),
		Signature: make([]byte, 65),
	}
	if err := a.BroadcastVote(vote); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := gotProp != nil && gotVote != nil
		mu.Unlock()
		if ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotProp == nil || gotProp.Height != 1 || gotProp.BlockHash != prop.BlockHash {
		t.Fatalf("proposal not delivered: %+v", gotProp)
	}
	if gotVote == nil || gotVote.Type != 1 || gotVote.Height != 1 {
		t.Fatalf("vote not delivered: %+v", gotVote)
	}
}

func waitPeers(t *testing.T, hosts ...*Host) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, h := range hosts {
			if h.PeerCount() < 1 {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("peers not connected in time")
}
