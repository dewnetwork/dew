package consensus

import (
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/crypto"
)

func TestActiveSetToValidatorSet(t *testing.T) {
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	b := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	vs, err := ActiveSetToValidatorSet([]native.Candidate{
		{Address: a, VotingPower: uint256.NewInt(100)},
		{Address: b, VotingPower: uint256.NewInt(300)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if vs.Size() != 2 || vs.TotalPower() != 400 {
		t.Fatalf("size=%d power=%d", vs.Size(), vs.TotalPower())
	}
	if vs.PowerOf(b) != 300 {
		t.Fatal(vs.PowerOf(b))
	}
}

func TestActiveSetToValidatorSet_Empty(t *testing.T) {
	if _, err := ActiveSetToValidatorSet(nil); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestShouldRotateEpoch(t *testing.T) {
	if ShouldRotateEpoch(0, 100) {
		t.Fatal("height 0")
	}
	if !ShouldRotateEpoch(100, 100) {
		t.Fatal("boundary")
	}
	if ShouldRotateEpoch(101, 100) {
		t.Fatal("mid")
	}
	if ShouldRotateEpoch(50, 0) {
		t.Fatal("zero epoch")
	}
}
