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

func TestStakingModule_DelegationPowerAndActiveSet(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1000)
	cfg.ActiveCap = 2
	m := NewStakingModule(s, cfg)

	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	del := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	other := crypto.MustHexToAddress("0x00000000000000000000000000000000000000cc")

	// Pure delegation without self-stake must not enter ActiveSet.
	if err := m.Delegate(val, del, uint256.NewInt(50_000)); err != nil {
		t.Fatal(err)
	}
	if len(m.ActiveSet()) != 0 {
		t.Fatal("pure delegation must not enter ActiveSet")
	}
	// Not a candidate yet → VP 0 even with del total.
	if !m.VotingPower(val).IsZero() {
		t.Fatal("non-candidate VP must be 0")
	}

	_ = m.Bond(val, uint256.NewInt(1000))
	if m.VotingPower(val).Uint64() != 51_000 {
		t.Fatalf("VP want 51000 got %s", m.VotingPower(val))
	}
	_ = m.Bond(other, uint256.NewInt(2000))
	set := m.ActiveSet()
	if len(set) != 2 || set[0].Address != val {
		t.Fatalf("rank: got %v", set)
	}
	if set[0].VotingPower.Uint64() != 51_000 {
		t.Fatalf("top power %s", set[0].VotingPower)
	}
}

func TestStakingModule_NoSelfDelegate(t *testing.T) {
	s := state.New(db.OpenTest(t))
	m := NewStakingModule(s, DefaultStakingConfig())
	a := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	if err := m.Delegate(a, a, uint256.NewInt(1)); err == nil {
		t.Fatal("expected self-delegate reject")
	}
}

func TestStakingModule_UndelegateQueue(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.UnbondSeconds = 100
	m := NewStakingModule(s, cfg)
	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	del := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	_ = m.Delegate(val, del, uint256.NewInt(100))
	const now uint64 = 1000
	if err := m.Undelegate(val, del, uint256.NewInt(40), now); err != nil {
		t.Fatal(err)
	}
	if m.Delegation(val, del).Uint64() != 60 {
		t.Fatal("live del")
	}
	if m.DelegatedTotal(val).Uint64() != 60 {
		t.Fatal("tot")
	}
	amt, unlock := m.PendingUndelegation(val, del)
	if amt.Uint64() != 40 || unlock != now+100 {
		t.Fatalf("pending %s %d", amt, unlock)
	}
	if _, err := m.WithdrawDelegation(val, del, now+50); err == nil {
		t.Fatal("early withdraw")
	}
	out, err := m.WithdrawDelegation(val, del, now+100)
	if err != nil || out.Uint64() != 40 {
		t.Fatalf("withdraw %v %v", out, err)
	}
}

func TestStakingModule_SetCommission(t *testing.T) {
	s := state.New(db.OpenTest(t))
	m := NewStakingModule(s, DefaultStakingConfig())
	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	if err := m.SetCommission(val, 1500); err != nil {
		t.Fatal(err)
	}
	if m.CommissionBps(val) != 1500 {
		t.Fatal("bps")
	}
	if err := m.SetCommission(val, MaxCommissionBps+1); err == nil {
		t.Fatal("over max")
	}
}

func TestStakingModule_DistributeTipAndClaim(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(100)
	m := NewStakingModule(s, cfg)

	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	del := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	_ = m.Bond(val, uint256.NewInt(100))
	_ = m.Delegate(val, del, uint256.NewInt(100))
	if err := m.SetCommission(val, 1000); err != nil { // 10%
		t.Fatal(err)
	}

	// T=1000: V = 1000 * (100*10000 + 100*1000) / (200*10000) = 1000 * 1.1e6 / 2e6 = 550
	// R = 450 to del pool
	tip := uint256.NewInt(1000)
	beforeVal := s.GetBalance(val)
	DistributeProposerIncome(s, cfg, val, tip, true)
	gotVal := new(uint256.Int).Sub(s.GetBalance(val), beforeVal)
	if gotVal.Uint64() != 550 {
		t.Fatalf("validator tip share got %s want 550", gotVal)
	}
	pend := m.PendingRewards(val, del)
	if pend.Uint64() != 450 {
		t.Fatalf("pending rewards got %s want 450", pend)
	}
	claimed, err := m.ClaimRewards(val, del)
	if err != nil || claimed.Uint64() != 450 {
		t.Fatalf("claim %v %v", claimed, err)
	}
	// Module held 450 for rewards; claim returns amount — transfer is caller's job.
	if !m.PendingRewards(val, del).IsZero() {
		t.Fatal("pending should clear after claim")
	}
}

func TestStakingModule_DistributeStakingOff(t *testing.T) {
	s := state.New(db.OpenTest(t))
	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	DistributeProposerIncome(s, DefaultStakingConfig(), val, uint256.NewInt(42), false)
	if s.GetBalance(val).Uint64() != 42 {
		t.Fatalf("want full tip to proposer, got %s", s.GetBalance(val))
	}
}

func TestStakingModule_SlashAndJailBurns(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(100)
	m := NewStakingModule(s, cfg)
	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	del := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")

	_ = m.Bond(val, uint256.NewInt(1000))
	_ = m.Delegate(val, del, uint256.NewInt(1000))
	// Escrow matches live stake.
	s.AddBalance(StakingModuleAddr, uint256.NewInt(2000))
	beforeMod := s.GetBalance(StakingModuleAddr)

	burned := m.SlashAndJail(val)
	// self 100% of 1000 + del 5% of 1000 = 1000 + 50 = 1050
	if burned.Uint64() != 1050 {
		t.Fatalf("burned %s want 1050", burned)
	}
	if !m.IsJailed(val) {
		t.Fatal("jailed")
	}
	if !m.SelfStake(val).IsZero() {
		t.Fatalf("self after 100%% slash: %s", m.SelfStake(val))
	}
	// effective del = 950
	if m.DelegatedTotal(val).Uint64() != 950 {
		t.Fatalf("del effective %s want 950", m.DelegatedTotal(val))
	}
	if m.Delegation(val, del).Uint64() != 950 {
		t.Fatalf("del pair %s want 950", m.Delegation(val, del))
	}
	modDrop := new(uint256.Int).Sub(beforeMod, s.GetBalance(StakingModuleAddr))
	if modDrop.Uint64() != 1050 {
		t.Fatalf("module burned %s", modDrop)
	}
	// Second slash: no extra burn
	if !m.SlashAndJail(val).IsZero() {
		t.Fatal("second slash must be zero")
	}
	if !m.VotingPower(val).IsZero() {
		t.Fatal("VP")
	}
}

func TestStakingModule_SelfOnlyTip(t *testing.T) {
	s := state.New(db.OpenTest(t))
	cfg := DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1)
	m := NewStakingModule(s, cfg)
	val := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	_ = m.Bond(val, uint256.NewInt(500))
	DistributeProposerIncome(s, cfg, val, uint256.NewInt(100), true)
	if s.GetBalance(val).Uint64() != 100 {
		t.Fatalf("self-only should take full tip, got %s", s.GetBalance(val))
	}
}
