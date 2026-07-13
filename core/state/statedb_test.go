package state

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

func TestStateDB_AccountRoundTrip(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)

	addr := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	s.SetBalance(addr, uint256.NewInt(1_000_000))
	s.SetNonce(addr, 7)

	root1, err := s.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if root1.IsZero() {
		t.Fatal("zero state root")
	}

	// New StateDB over same DB must load persisted account.
	s2 := New(mdb)
	if s2.GetNonce(addr) != 7 {
		t.Fatalf("nonce = %d, want 7", s2.GetNonce(addr))
	}
	if s2.GetBalance(addr).Uint64() != 1_000_000 {
		t.Fatalf("balance = %s", s2.GetBalance(addr))
	}

	// Root of reloaded empty-dirty state over same data: load into cache then root.
	s2.GetOrNewAccount(addr) // already loaded via GetNonce
	// Force account in cache for root: read again
	_ = s2.GetBalance(addr)
	// After commit, cache has the account; IntermediateRoot uses cache.
	// Reload path: only accounts touched enter cache. Touch + recompute after re-put.
	root2, err := s2.IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}
	// root2 may differ if cache only partially loaded — commit again after touch
	s2.SetNonce(addr, 7) // dirty same value
	root3, err := s2.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if root3 != root1 {
		t.Fatalf("state root drift: %s vs %s (mid %s)", root1, root3, root2)
	}
}

func TestStateDB_StorageAndCode(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)

	addr := crypto.MustHexToAddress("0x0000000000000000000000000000000000000001")
	slot := types.BytesToHash([]byte{1})
	val := types.BytesToHash([]byte{0xab})
	s.SetState(addr, slot, val)
	s.SetCode(addr, []byte{0x60, 0x00, 0x60, 0x00, 0xf3})

	if got := s.GetState(addr, slot); got != val {
		t.Fatalf("storage = %s, want %s", got, val)
	}
	if s.GetCodeHash(addr) == types.EmptyCodeHash {
		t.Fatal("expected code hash")
	}
	if len(s.GetCode(addr)) != 5 {
		t.Fatalf("code len = %d", len(s.GetCode(addr)))
	}

	root, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}

	s2 := New(mdb)
	// touch account to load
	if !s2.Exist(addr) {
		t.Fatal("account missing after commit")
	}
	// load storage from db
	if got := s2.GetState(addr, slot); got != val {
		t.Fatalf("persisted storage = %s", got)
	}
	if len(s2.GetCode(addr)) != 5 {
		t.Fatal("code not persisted")
	}
	_ = root
}

func TestStateDB_RootStable(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)
	a1 := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	a2 := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	s.AddBalance(a1, uint256.MustFromBig(big.NewInt(100)))
	s.AddBalance(a2, uint256.MustFromBig(big.NewInt(200)))
	r1, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}

	mdb2 := db.OpenTest(t)
	sB := New(mdb2)
	// Insert in reverse order — root must match.
	sB.AddBalance(a2, uint256.MustFromBig(big.NewInt(200)))
	sB.AddBalance(a1, uint256.MustFromBig(big.NewInt(100)))
	r2, err := sB.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if r1 != r2 {
		t.Fatalf("root order-dependent: %s vs %s", r1, r2)
	}
}
