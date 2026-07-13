package native

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

func TestStakingModule_ActiveSetRanking(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1000)
	cfg.ActiveCap = 2
	m := NewStakingModule(s, cfg)

	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	b := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	c := crypto.MustHexToAddress("0x00000000000000000000000000000000000000cc")

	_ = m.Bond(a, uint256.NewInt(5000))
	_ = m.Bond(b, uint256.NewInt(9000))
	_ = m.Bond(c, uint256.NewInt(7000))

	set := m.ActiveSet()
	if len(set) != 2 {
		t.Fatalf("cap 2, got %d", len(set))
	}
	if set[0].Address != b || set[1].Address != c {
		t.Fatalf("order: %v %v", set[0].Address.Hex(), set[1].Address.Hex())
	}
}

func TestStakingModule_JailDropsPower(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1)
	m := NewStakingModule(s, cfg)
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	_ = m.Bond(a, uint256.NewInt(100))
	if m.VotingPower(a).Uint64() != 100 {
		t.Fatal("power")
	}
	m.Jail(a)
	if !m.IsJailed(a) || !m.VotingPower(a).IsZero() {
		t.Fatal("jail should zero power")
	}
	if len(m.ActiveSet()) != 0 {
		t.Fatal("active")
	}
}

func TestStakingModule_Unbond(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(50)
	m := NewStakingModule(s, cfg)
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	_ = m.Bond(a, uint256.NewInt(100))
	out, err := m.Unbond(a, uint256.NewInt(60))
	if err != nil {
		t.Fatal(err)
	}
	if out.Uint64() != 60 {
		t.Fatal(out)
	}
	if m.SelfStake(a).Uint64() != 40 {
		t.Fatal(m.SelfStake(a))
	}
	if m.IsCandidate(a) {
		t.Fatal("should drop candidate")
	}
}
