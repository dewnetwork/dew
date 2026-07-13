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

func TestStakingModule_UnbondQueuesAndWithdraw(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(50)
	cfg.UnbondSeconds = 100
	m := NewStakingModule(s, cfg)
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	_ = m.Bond(a, uint256.NewInt(100))

	const now uint64 = 1_000
	if err := m.Unbond(a, uint256.NewInt(60), now); err != nil {
		t.Fatal(err)
	}
	if m.SelfStake(a).Uint64() != 40 {
		t.Fatalf("stake after unbond: %s", m.SelfStake(a))
	}
	if m.IsCandidate(a) {
		t.Fatal("should drop candidate")
	}
	amt, unlock := m.PendingUnbond(a)
	if amt.Uint64() != 60 || unlock != now+100 {
		t.Fatalf("pending amt=%s unlock=%d", amt, unlock)
	}

	// Too early
	if _, err := m.Withdraw(a, now+50); err == nil {
		t.Fatal("expected early withdraw to fail")
	}
	// Mature
	out, err := m.Withdraw(a, now+100)
	if err != nil {
		t.Fatal(err)
	}
	if out.Uint64() != 60 {
		t.Fatalf("withdraw %s", out)
	}
	amt, unlock = m.PendingUnbond(a)
	if !amt.IsZero() || unlock != 0 {
		t.Fatalf("pending should clear: amt=%s unlock=%d", amt, unlock)
	}
	if _, err := m.Withdraw(a, now+200); err == nil {
		t.Fatal("second withdraw should fail")
	}
}

func TestStakingModule_UnbondZeroPeriodImmediate(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1)
	cfg.UnbondSeconds = 0
	m := NewStakingModule(s, cfg)
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	_ = m.Bond(a, uint256.NewInt(10))
	if err := m.Unbond(a, uint256.NewInt(10), 50); err != nil {
		t.Fatal(err)
	}
	out, err := m.Withdraw(a, 50)
	if err != nil {
		t.Fatal(err)
	}
	if out.Uint64() != 10 {
		t.Fatal(out)
	}
}

func TestStakingModule_UnbondStacksPending(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1)
	cfg.UnbondSeconds = 10
	m := NewStakingModule(s, cfg)
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	_ = m.Bond(a, uint256.NewInt(100))
	_ = m.Unbond(a, uint256.NewInt(30), 100) // unlock 110
	_ = m.Unbond(a, uint256.NewInt(20), 105) // unlock max(110, 115)=115
	amt, unlock := m.PendingUnbond(a)
	if amt.Uint64() != 50 {
		t.Fatalf("amt=%s", amt)
	}
	if unlock != 115 {
		t.Fatalf("unlock=%d want 115", unlock)
	}
}
