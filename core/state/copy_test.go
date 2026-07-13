package state

import (
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

func TestStateDB_CopyIsolation(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)
	a := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	s.SetBalance(a, uint256.NewInt(1000))
	if _, err := s.Commit(); err != nil {
		t.Fatal(err)
	}

	c := s.Copy()
	c.SetBalance(a, uint256.NewInt(42))
	if s.GetBalance(a).Uint64() != 1000 {
		t.Fatalf("parent mutated: %s", s.GetBalance(a))
	}
	if c.GetBalance(a).Uint64() != 42 {
		t.Fatalf("copy bal = %s", c.GetBalance(a))
	}
}

func TestStateDB_ApplyOverlay(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)
	a := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	b := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	s.SetBalance(a, uint256.NewInt(1000))
	if _, err := s.Commit(); err != nil {
		t.Fatal(err)
	}

	c := s.Copy()
	c.SetBalance(a, uint256.NewInt(900))
	c.SetBalance(b, uint256.NewInt(100))
	slot := types.BytesToHash([]byte{1})
	c.SetState(a, slot, types.BytesToHash([]byte{9}))

	s.ApplyOverlay(c)
	if s.GetBalance(a).Uint64() != 900 {
		t.Fatalf("a bal = %s", s.GetBalance(a))
	}
	if s.GetBalance(b).Uint64() != 100 {
		t.Fatalf("b bal = %s", s.GetBalance(b))
	}
	if s.GetState(a, slot) != types.BytesToHash([]byte{9}) {
		t.Fatalf("slot = %x", s.GetState(a, slot))
	}
}

func TestAccessSet_Conflicts(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)
	a := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	b := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	s.StartAccessTracking()
	_ = s.GetBalance(a)
	s.SetBalance(b, uint256.NewInt(1))
	as := s.TakeAccessSet()

	earlier := map[AccessKey]struct{}{AccountKey(a): {}}
	if !as.ConflictsWith(earlier) {
		t.Fatal("expected conflict on read of a")
	}
	earlier2 := map[AccessKey]struct{}{AccountKey(b): {}}
	if !as.ConflictsWith(earlier2) {
		t.Fatal("expected conflict on write of b")
	}
	earlier3 := map[AccessKey]struct{}{
		AccountKey(crypto.MustHexToAddress("0x0000000000000000000000000000000000000001")): {},
	}
	if as.ConflictsWith(earlier3) {
		t.Fatal("unexpected conflict")
	}
}
