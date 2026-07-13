package state

import (
	"bytes"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

func TestSMT_EmptyRoot(t *testing.T) {
	if ComputeSMTRoot(nil) != EmptySMTRoot() {
		t.Fatal("nil leaves")
	}
	if ComputeSMTRoot([]leaf{}) != EmptySMTRoot() {
		t.Fatal("empty slice")
	}
	s := New(db.OpenTest(t))
	root, err := s.IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != EmptySMTRoot() {
		t.Fatalf("empty statedb root %x want %x", root, EmptySMTRoot())
	}
}

func TestSMT_SingleAccount(t *testing.T) {
	mdb := db.OpenTest(t)
	s := New(mdb)
	addr := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	s.SetBalance(addr, uint256.NewInt(42))
	root1, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if root1 == EmptySMTRoot() || root1.IsZero() {
		t.Fatal("expected non-empty root")
	}

	// Independent node with same pre-state mutation
	s2 := New(db.OpenTest(t))
	s2.SetBalance(addr, uint256.NewInt(42))
	root2, err := s2.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if root1 != root2 {
		t.Fatalf("roots diverge %x vs %x", root1, root2)
	}
}

func TestSMT_StorageSlots(t *testing.T) {
	s := New(db.OpenTest(t))
	addr := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	s.SetBalance(addr, uint256.NewInt(1))
	slot0 := types.BytesToHash([]byte{0x01})
	slot1 := types.BytesToHash([]byte{0x02})
	s.SetState(addr, slot0, types.BytesToHash([]byte{0xaa}))
	s.SetState(addr, slot1, types.BytesToHash([]byte{0xbb}))
	root, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}

	sB := New(db.OpenTest(t))
	sB.SetBalance(addr, uint256.NewInt(1))
	sB.SetState(addr, slot1, types.BytesToHash([]byte{0xbb}))
	sB.SetState(addr, slot0, types.BytesToHash([]byte{0xaa})) // reverse write order
	rootB, err := sB.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if root != rootB {
		t.Fatalf("storage order affects root: %x vs %x", root, rootB)
	}
}

func TestSMT_DeleteEmptyAccount(t *testing.T) {
	s := New(db.OpenTest(t))
	addr := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	s.SetBalance(addr, uint256.NewInt(100))
	slot := types.BytesToHash([]byte{0x01})
	s.SetState(addr, slot, types.BytesToHash([]byte{0xff}))
	rootWith, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}

	// Zero storage slot (delete leaf) + zero balance → different commitment.
	s.SetState(addr, slot, types.Hash{})
	s.SetBalance(addr, uint256.NewInt(0))
	rootAfter, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if rootWith == rootAfter {
		t.Fatal("delete/zero should change root")
	}

	// Fresh empty DB still empty root
	empty, err := New(db.OpenTest(t)).IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}
	if empty != EmptySMTRoot() {
		t.Fatal("empty root mismatch")
	}
}

func TestSMT_CodeLeaf(t *testing.T) {
	s := New(db.OpenTest(t))
	addr := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	code := []byte{0x60, 0x00, 0x60, 0x00, 0xf3}
	s.SetCode(addr, code)
	s.SetBalance(addr, uint256.NewInt(0))
	root1, err := s.Commit()
	if err != nil {
		t.Fatal(err)
	}
	s2 := New(db.OpenTest(t))
	s2.SetCode(addr, code)
	s2.SetBalance(addr, uint256.NewInt(0))
	root2, err := s2.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if root1 != root2 {
		t.Fatalf("code roots %x vs %x", root1, root2)
	}
	if bytes.Equal(root1.Bytes(), EmptySMTRoot().Bytes()) {
		t.Fatal("code should not be empty root")
	}
}

func TestSMT_DeterministicPath(t *testing.T) {
	leaves := []leaf{
		{key: []byte("a"), val: []byte("1")},
		{key: []byte("b"), val: []byte("2")},
		{key: []byte("c"), val: []byte("3")},
	}
	r1 := ComputeSMTRoot(leaves)
	// reverse
	rev := []leaf{leaves[2], leaves[1], leaves[0]}
	r2 := ComputeSMTRoot(rev)
	if r1 != r2 {
		t.Fatal("order dependent")
	}
}
