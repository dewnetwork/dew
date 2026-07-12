package p2p

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dewnetwork/dew/crypto"
)

func TestPeerStore_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")

	id := crypto.MustHexToAddress("0x1111111111111111111111111111111111111111")
	s := NewPeerStore(10)
	s.Remember(id, "127.0.0.1:30303")
	s.AddBanScore(id, 3)

	if err := s.SaveToFile(path); err != nil {
		t.Fatal(err)
	}

	s2 := NewPeerStore(10)
	if err := s2.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	k, ok := s2.KnownByID(id)
	if !ok {
		t.Fatal("missing peer after load")
	}
	if k.Addr != "127.0.0.1:30303" {
		t.Fatalf("addr=%s", k.Addr)
	}
	if k.BanScore != 3 {
		t.Fatalf("banScore=%d want 3", k.BanScore)
	}
}

func TestPeerStore_LoadMissingOK(t *testing.T) {
	s := NewPeerStore(10)
	if err := s.LoadFromFile(filepath.Join(t.TempDir(), "nope.json")); err != nil {
		t.Fatal(err)
	}
}

func TestPeerStore_LoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewPeerStore(10)
	if err := s.LoadFromFile(path); err == nil {
		t.Fatal("expected corrupt parse error")
	}
}

func TestPeerStore_EvictOlderThan(t *testing.T) {
	idOld := crypto.MustHexToAddress("0x2222222222222222222222222222222222222222")
	idBoot := crypto.MustHexToAddress("0x3333333333333333333333333333333333333333")
	s := NewPeerStore(10)
	s.mu.Lock()
	s.known[idOld] = KnownPeer{ID: idOld, Addr: "10.0.0.1:1", LastSeen: time.Now().Add(-8 * 24 * time.Hour)}
	s.known[idBoot] = KnownPeer{ID: idBoot, Addr: "10.0.0.2:2", LastSeen: time.Now().Add(-8 * 24 * time.Hour)}
	s.mu.Unlock()

	n := s.EvictOlderThan(DefaultPeerTTL, map[string]struct{}{"10.0.0.2:2": {}})
	if n != 1 {
		t.Fatalf("evicted=%d want 1", n)
	}
	if _, ok := s.KnownByID(idOld); ok {
		t.Fatal("old peer should be gone")
	}
	if _, ok := s.KnownByID(idBoot); !ok {
		t.Fatal("bootnode addr should be kept")
	}
}

func TestPeerStore_DialableSkipsBan(t *testing.T) {
	id := crypto.MustHexToAddress("0x4444444444444444444444444444444444444444")
	s := NewPeerStore(10)
	s.Remember(id, "127.0.0.1:9")
	s.AddBanScore(id, BanDialThreshold)
	if got := s.Dialable(); len(got) != 0 {
		t.Fatalf("dialable=%v want empty", got)
	}
}
