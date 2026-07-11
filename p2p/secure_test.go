package p2p

import (
	"math/big"
	"testing"
	"time"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func TestEncryptedMesh_GossipAndSync(t *testing.T) {
	gHash := types.Keccak256Hash([]byte("enc2-g"))
	gRaw := []byte("g")

	srcChain := NewMemoryChain(gHash, gRaw)
	for i := uint64(1); i <= 3; i++ {
		srcChain.AddBlock(i, types.Keccak256Hash([]byte{byte(i)}), []byte{byte(i)})
	}
	midChain := NewMemoryChain(gHash, gRaw)
	dstChain := NewMemoryChain(gHash, gRaw)

	src := newEncryptedHost(t, srcChain, AppHandlers{})
	mid := newEncryptedHost(t, midChain, AppHandlers{
		OnBlock: func(number uint64, hash types.Hash, raw []byte, from PeerID) error {
			midChain.AddBlock(number, hash, raw)
			return nil
		},
	})
	dst := newEncryptedHost(t, dstChain, AppHandlers{
		OnBlock: func(number uint64, hash types.Hash, raw []byte, from PeerID) error {
			dstChain.AddBlock(number, hash, raw)
			return nil
		},
	})

	pMid, err := mid.Dial(src.ListenAddr())
	if err != nil {
		t.Fatal(err)
	}
	if !pMid.Encrypted() {
		t.Fatal("mid↔src not encrypted")
	}
	pDst, err := dst.Dial(mid.ListenAddr())
	if err != nil {
		t.Fatal(err)
	}
	if !pDst.Encrypted() {
		t.Fatal("dst↔mid not encrypted")
	}

	// Wait mesh
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if src.PeerCount() >= 1 && mid.PeerCount() >= 2 && dst.PeerCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Sync mid from src, then dst from mid — same committed height chain.
	if pMid.Height < 3 {
		pMid.Height = srcChain.Height()
	}
	if err := mid.SyncFromPeer(pMid); err != nil {
		t.Fatal(err)
	}
	if midChain.Height() != 3 {
		t.Fatalf("mid height %d want 3", midChain.Height())
	}
	// Refresh dst peer height (handshake was at 0)
	pDst.Height = midChain.Height()
	if err := dst.SyncFromPeer(pDst); err != nil {
		t.Fatal(err)
	}
	if dstChain.Height() != 3 {
		t.Fatalf("dst height %d want 3", dstChain.Height())
	}
	// All three report the same tip height after sync.
	if srcChain.Height() != midChain.Height() || midChain.Height() != dstChain.Height() {
		t.Fatalf("heights diverge src=%d mid=%d dst=%d", srcChain.Height(), midChain.Height(), dstChain.Height())
	}
}

func TestCleartextRequiresFlag(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain := NewMemoryChain(types.Hash{}, nil)
	_, err := NewHost(Config{
		PrivateKey:     key,
		ChainID:        big.NewInt(1),
		Encrypt:        false,
		AllowCleartext: true,
	}, chain, chain, AppHandlers{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncryptVsCleartextMismatch(t *testing.T) {
	g := NewMemoryChain(types.Keccak256Hash([]byte("m")), []byte("m"))
	enc := newEncryptedHost(t, g, AppHandlers{})
	clear := newCleartextHost(t, NewMemoryChain(types.Keccak256Hash([]byte("m")), []byte("m")), AppHandlers{})
	if _, err := clear.Dial(enc.ListenAddr()); err == nil {
		t.Fatal("expected cleartext→encrypted dial to fail")
	}
}

func newEncryptedHost(t *testing.T, chain *MemoryChain, handlers AppHandlers) *Host {
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
		ChainID:    big.NewInt(2205),
		ListenAddr: "127.0.0.1:0",
		MaxPeers:   10,
		Encrypt:    true,
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

func newCleartextHost(t *testing.T, chain *MemoryChain, handlers AppHandlers) *Host {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHost(Config{
		PrivateKey:     key,
		ChainID:        big.NewInt(2205),
		ListenAddr:     "127.0.0.1:0",
		MaxPeers:       10,
		Encrypt:        false,
		AllowCleartext: true,
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
