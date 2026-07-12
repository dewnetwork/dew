package p2p

import (
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func newRedialHost(t *testing.T, path string, boot []string) *Host {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	chain := NewMemoryChain(types.Hash{}, nil)
	on := true
	h, err := NewHost(Config{
		PrivateKey:    key,
		ChainID:       big.NewInt(2205),
		ListenAddr:    "127.0.0.1:0",
		MaxPeers:      10,
		PeerStorePath: path,
		Bootnodes:     boot,
		Redial:        &on,
	}, chain, chain, AppHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// TestAutoRedial_AfterDisconnect — D3b: peer drop → background redial without helper.
func TestAutoRedial_AfterDisconnect(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a-peers.json")

	b := newRedialHost(t, "", nil)
	a := newRedialHost(t, aPath, []string{b.ListenAddr()})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a.PeerCount() >= 1 && b.PeerCount() >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if a.PeerCount() < 1 {
		t.Fatalf("a peers=%d want >=1", a.PeerCount())
	}

	// Drop the session from A's side.
	var peer *Peer
	for _, p := range a.Store().Active() {
		peer = p
		break
	}
	if peer == nil {
		t.Fatal("no active peer on a")
	}
	_ = peer.Close()

	// Wait until disconnected.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.PeerCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Auto-redial should restore the session.
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if a.PeerCount() >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("a peers=%d after redial window", a.PeerCount())
}

// TestPeerStore_ReloadAndRedial — restart loads peers.json and dials.
func TestPeerStore_ReloadAndRedial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")

	b := newRedialHost(t, "", nil)
	// First host remembers B via dial, then exits (flush peers.json).
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	chain := NewMemoryChain(types.Hash{}, nil)
	off := false
	a1, err := NewHost(Config{
		PrivateKey:    key,
		ChainID:       big.NewInt(2205),
		ListenAddr:    "127.0.0.1:0",
		MaxPeers:      10,
		PeerStorePath: path,
		Redial:        &off, // manual dial only for first session
	}, chain, chain, AppHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a1.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := a1.Dial(b.ListenAddr()); err != nil {
		t.Fatal(err)
	}
	// Ensure Remember landed before close flush.
	time.Sleep(50 * time.Millisecond)
	if err := a1.Close(); err != nil {
		t.Fatal(err)
	}

	// New process with same key path: load + redial to B.
	on := true
	a2, err := NewHost(Config{
		PrivateKey:    key,
		ChainID:       big.NewInt(2205),
		ListenAddr:    "127.0.0.1:0",
		MaxPeers:      10,
		PeerStorePath: path,
		Redial:        &on,
	}, chain, chain, AppHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a2.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a2.Close() })

	if _, ok := a2.Store().KnownByID(b.ID()); !ok {
		t.Fatal("reloaded store missing b")
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if a2.PeerCount() >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("a2 peers=%d after reload redial", a2.PeerCount())
}
