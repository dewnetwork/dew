package wallet

import (
	"path/filepath"
	"testing"

	"github.com/dewnetwork/dew/crypto"
)

func TestStore_CreateAndList(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "keystore"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if got := store.List(); len(got) != 0 {
		t.Fatalf("List empty = %v, want empty", got)
	}

	pass := "test-passphrase-a1"
	addr, err := store.Create(pass)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if addr.IsZero() {
		t.Fatal("Create returned zero address")
	}
	if len(addr.Bytes()) != 20 {
		t.Fatalf("address len = %d", len(addr.Bytes()))
	}

	list := store.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}
	if list[0] != addr {
		t.Fatalf("List[0] = %s, want %s", list[0].Hex(), addr.Hex())
	}

	// Second wallet
	addr2, err := store.Create(pass)
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	if addr2 == addr {
		t.Fatal("second address should differ")
	}
	if len(store.List()) != 2 {
		t.Fatalf("List len = %d, want 2", len(store.List()))
	}
}

func TestStore_Unlock_RoundTripSign(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "keystore"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	pass := "unlock-me"
	addr, err := store.Create(pass)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	key, err := store.Unlock(addr, pass)
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	derived := crypto.PubkeyToAddress(&key.PublicKey)
	if derived != addr {
		t.Fatalf("unlocked key address %s != stored %s", derived.Hex(), addr.Hex())
	}

	digest := crypto.Keccak256([]byte("wallet unlock test"))
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !crypto.VerifySignature(crypto.FromECDSAPub(&key.PublicKey), digest, sig[:64]) {
		t.Fatal("signature verify failed after unlock")
	}

	if _, err := store.Unlock(addr, "wrong"); err == nil {
		t.Fatal("expected decrypt error for wrong passphrase")
	}
}
