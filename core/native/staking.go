// Package native hosts Dew-native execution paths (DewTx + staking module state).
package native

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// StakingModuleAddr is the system account holding staking storage (same as precompile 0x102).
var StakingModuleAddr = crypto.MustHexToAddress("0x0000000000000000000000000000000000000102")

// Storage slot layout (under StakingModuleAddr):
//
//	keccak256("dew/stake/v1/self" || addr)       → self-stake amount (32-byte big-endian)
//	keccak256("dew/stake/v1/jailed" || addr)     → 1 if jailed
//	keccak256("dew/stake/v1/cand" || addr)       → 1 if candidate registered
//	keccak256("dew/stake/v1/candlen")            → candidate count
//	keccak256("dew/stake/v1/candi" || uint64)    → candidate address at index
//	keccak256("dew/stake/v1/unbondAmt" || addr)  → pending unbond amount (escrowed)
//	keccak256("dew/stake/v1/unbondAt" || addr)   → unlock unix timestamp (seconds)
//	keccak256("dew/stake/v1/del" || val || del)  → live delegation **shares**
//	keccak256("dew/stake/v1/delTot" || val)      → total delegated **shares**
//	keccak256("dew/stake/v1/comm" || val)        → commission bps (0–10000)
//	keccak256("dew/stake/v1/delUnbondAmt" || val || del) → pending undelegation (wei)
//	keccak256("dew/stake/v1/delUnbondAt" || val || del)  → unlock unix
//	keccak256("dew/stake/v1/delRate" || val)     → del exchange rate (0 ⇒ 1e18)
//	keccak256("dew/stake/v1/rewIdx" || val)      → reward per share × 1e18
//	keccak256("dew/stake/v1/rewDebt" || val || del) → reward debt
//	keccak256("dew/stake/v1/rewPend" || val || del) → settled unclaimed rewards (wei)
const (
	stakeDomainSelf         = "dew/stake/v1/self"
	stakeDomainJailed       = "dew/stake/v1/jailed"
	stakeDomainCand         = "dew/stake/v1/cand"
	stakeDomainLen          = "dew/stake/v1/candlen"
	stakeDomainIdx          = "dew/stake/v1/candi"
	stakeDomainUnbondAmt    = "dew/stake/v1/unbondAmt"
	stakeDomainUnbondAt     = "dew/stake/v1/unbondAt"
	stakeDomainDel          = "dew/stake/v1/del"
	stakeDomainDelTot       = "dew/stake/v1/delTot"
	stakeDomainComm         = "dew/stake/v1/comm"
	stakeDomainDelUnbondAmt = "dew/stake/v1/delUnbondAmt"
	stakeDomainDelUnbondAt  = "dew/stake/v1/delUnbondAt"
	stakeDomainDelRate      = "dew/stake/v1/delRate"
	stakeDomainRewIdx       = "dew/stake/v1/rewIdx"
	stakeDomainRewDebt      = "dew/stake/v1/rewDebt"
	stakeDomainRewPend      = "dew/stake/v1/rewPend"

	// MaxCommissionBps is 100% in basis points.
	MaxCommissionBps uint64 = 10_000
)

// StakingConfig is runtime staking parameters.
type StakingConfig struct {
	MinSelfStake   *big.Int
	ActiveCap      uint64
	EpochLength    uint64
	UnbondSeconds  uint64
}

// DefaultStakingConfig returns tentative C4 defaults.
func DefaultStakingConfig() StakingConfig {
	return StakingConfig{
		MinSelfStake:  params.MinValidatorStakeWei(),
		ActiveCap:     params.DefaultActiveValidatorCap,
		EpochLength:   params.DefaultEpochLengthBlocks,
		UnbondSeconds: params.DefaultUnbondingPeriodSeconds,
	}
}

// StakingModule reads/writes staking state via flat StateDB storage.
type StakingModule struct {
	db  *state.StateDB
	cfg StakingConfig
}

// NewStakingModule binds to statedb.
func NewStakingModule(db *state.StateDB, cfg StakingConfig) *StakingModule {
	if cfg.MinSelfStake == nil {
		cfg.MinSelfStake = params.MinValidatorStakeWei()
	}
	if cfg.ActiveCap == 0 {
		cfg.ActiveCap = params.DefaultActiveValidatorCap
	}
	return &StakingModule{db: db, cfg: cfg}
}

func slotHash(parts ...[]byte) types.Hash {
	return types.BytesToHash(crypto.Keccak256(parts...))
}

func (m *StakingModule) selfSlot(addr crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainSelf), addr.Bytes())
}
func (m *StakingModule) jailedSlot(addr crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainJailed), addr.Bytes())
}
func (m *StakingModule) candSlot(addr crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainCand), addr.Bytes())
}
func (m *StakingModule) lenSlot() types.Hash {
	return slotHash([]byte(stakeDomainLen))
}
func (m *StakingModule) idxSlot(i uint64) types.Hash {
	var be [8]byte
	for b := 0; b < 8; b++ {
		be[7-b] = byte(i >> (8 * b))
	}
	return slotHash([]byte(stakeDomainIdx), be[:])
}
func (m *StakingModule) unbondAmtSlot(addr crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainUnbondAmt), addr.Bytes())
}
func (m *StakingModule) unbondAtSlot(addr crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainUnbondAt), addr.Bytes())
}
func (m *StakingModule) delSlot(val, del crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainDel), val.Bytes(), del.Bytes())
}
func (m *StakingModule) delTotSlot(val crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainDelTot), val.Bytes())
}
func (m *StakingModule) commSlot(val crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainComm), val.Bytes())
}
func (m *StakingModule) delUnbondAmtSlot(val, del crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainDelUnbondAmt), val.Bytes(), del.Bytes())
}
func (m *StakingModule) delUnbondAtSlot(val, del crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainDelUnbondAt), val.Bytes(), del.Bytes())
}
func (m *StakingModule) delRateSlot(val crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainDelRate), val.Bytes())
}
func (m *StakingModule) rewIdxSlot(val crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainRewIdx), val.Bytes())
}
func (m *StakingModule) rewDebtSlot(val, del crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainRewDebt), val.Bytes(), del.Bytes())
}
func (m *StakingModule) rewPendSlot(val, del crypto.Address) types.Hash {
	return slotHash([]byte(stakeDomainRewPend), val.Bytes(), del.Bytes())
}

func stakePrecision() *uint256.Int {
	return uint256.NewInt(params.StakeRatePrecision)
}

func hashToU256(h types.Hash) *uint256.Int {
	return new(uint256.Int).SetBytes(h.Bytes())
}

func u256ToHash(v *uint256.Int) types.Hash {
	if v == nil {
		return types.Hash{}
	}
	b := v.Bytes32()
	return types.BytesToHash(b[:])
}

// SelfStake returns bonded self-stake for addr.
func (m *StakingModule) SelfStake(addr crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.selfSlot(addr)))
}

// IsJailed reports jail status.
func (m *StakingModule) IsJailed(addr crypto.Address) bool {
	return !m.db.GetState(StakingModuleAddr, m.jailedSlot(addr)).IsZero()
}

// IsCandidate reports whether addr registered.
func (m *StakingModule) IsCandidate(addr crypto.Address) bool {
	return !m.db.GetState(StakingModuleAddr, m.candSlot(addr)).IsZero()
}

// DelRate returns the delegated-stake exchange rate (shares → wei). Zero storage ⇒ 1e18.
func (m *StakingModule) DelRate(validator crypto.Address) *uint256.Int {
	r := hashToU256(m.db.GetState(StakingModuleAddr, m.delRateSlot(validator)))
	if r.IsZero() {
		return stakePrecision()
	}
	return r
}

// DelegationShares returns raw share balance (pre-rate).
func (m *StakingModule) DelegationShares(validator, delegator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.delSlot(validator, delegator)))
}

// DelegatedTotalShares returns sum of live delegation shares.
func (m *StakingModule) DelegatedTotalShares(validator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.delTotSlot(validator)))
}

// sharesToWei converts shares to effective wei at validator rate.
func (m *StakingModule) sharesToWei(validator crypto.Address, shares *uint256.Int) *uint256.Int {
	if shares == nil || shares.IsZero() {
		return uint256.NewInt(0)
	}
	// wei = shares * rate / 1e18
	return new(uint256.Int).Div(new(uint256.Int).Mul(shares, m.DelRate(validator)), stakePrecision())
}

// weiToShares converts effective wei to shares at validator rate (floor).
func (m *StakingModule) weiToShares(validator crypto.Address, wei *uint256.Int) *uint256.Int {
	if wei == nil || wei.IsZero() {
		return uint256.NewInt(0)
	}
	rate := m.DelRate(validator)
	// shares = wei * 1e18 / rate
	return new(uint256.Int).Div(new(uint256.Int).Mul(wei, stakePrecision()), rate)
}

// Delegation returns effective live delegated wei (shares × rate).
func (m *StakingModule) Delegation(validator, delegator crypto.Address) *uint256.Int {
	return m.sharesToWei(validator, m.DelegationShares(validator, delegator))
}

// DelegatedTotal returns effective sum of live delegations (wei).
func (m *StakingModule) DelegatedTotal(validator crypto.Address) *uint256.Int {
	return m.sharesToWei(validator, m.DelegatedTotalShares(validator))
}

// CommissionBps returns commission rate in basis points (0–10000).
func (m *StakingModule) CommissionBps(validator crypto.Address) uint64 {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.commSlot(validator))).Uint64()
}

// PendingUndelegation returns queued undelegation for (validator, delegator).
func (m *StakingModule) PendingUndelegation(validator, delegator crypto.Address) (amount *uint256.Int, unlockAt uint64) {
	amount = hashToU256(m.db.GetState(StakingModuleAddr, m.delUnbondAmtSlot(validator, delegator)))
	unlockAt = hashToU256(m.db.GetState(StakingModuleAddr, m.delUnbondAtSlot(validator, delegator))).Uint64()
	return amount, unlockAt
}

// VotingPower is self-stake + delegated total if candidate and not jailed, else 0.
func (m *StakingModule) VotingPower(addr crypto.Address) *uint256.Int {
	if m.IsJailed(addr) || !m.IsCandidate(addr) {
		return uint256.NewInt(0)
	}
	return new(uint256.Int).Add(m.SelfStake(addr), m.DelegatedTotal(addr))
}

// Bond adds amount (already transferred to module as CALLVALUE) to self-stake
// and registers as candidate when >= min self-stake.
func (m *StakingModule) Bond(addr crypto.Address, amount *uint256.Int) error {
	if amount == nil || amount.IsZero() {
		return fmt.Errorf("staking: zero bond")
	}
	cur := m.SelfStake(addr)
	next := new(uint256.Int).Add(cur, amount)
	m.db.SetState(StakingModuleAddr, m.selfSlot(addr), u256ToHash(next))

	min := uint256.MustFromBig(m.cfg.MinSelfStake)
	if next.Cmp(min) >= 0 && !m.IsCandidate(addr) {
		m.registerCandidate(addr)
	}
	return nil
}

func (m *StakingModule) registerCandidate(addr crypto.Address) {
	if m.IsCandidate(addr) {
		return
	}
	m.db.SetState(StakingModuleAddr, m.candSlot(addr), types.BytesToHash([]byte{1}))
	n := hashToU256(m.db.GetState(StakingModuleAddr, m.lenSlot())).Uint64()
	// store address in slot as right-aligned 20 bytes
	var h types.Hash
	copy(h[12:], addr[:])
	m.db.SetState(StakingModuleAddr, m.idxSlot(n), h)
	m.db.SetState(StakingModuleAddr, m.lenSlot(), u256ToHash(uint256.NewInt(n+1)))
}

// PendingUnbond returns queued unbond amount and unlock unix timestamp (0 if none).
func (m *StakingModule) PendingUnbond(addr crypto.Address) (amount *uint256.Int, unlockAt uint64) {
	amount = hashToU256(m.db.GetState(StakingModuleAddr, m.unbondAmtSlot(addr)))
	unlockAt = hashToU256(m.db.GetState(StakingModuleAddr, m.unbondAtSlot(addr))).Uint64()
	return amount, unlockAt
}

// Unbond reduces self-stake and queues amount in the unbonding escrow until
// blockTime + UnbondSeconds (D3c). Funds stay at module address; call Withdraw after unlock.
// now is the current block timestamp (unix seconds).
func (m *StakingModule) Unbond(addr crypto.Address, amount *uint256.Int, now uint64) error {
	if amount == nil || amount.IsZero() {
		return fmt.Errorf("staking: zero unbond")
	}
	if m.IsJailed(addr) {
		return fmt.Errorf("staking: jailed")
	}
	cur := m.SelfStake(addr)
	if cur.Cmp(amount) < 0 {
		return fmt.Errorf("staking: insufficient stake")
	}
	next := new(uint256.Int).Sub(cur, amount)
	m.db.SetState(StakingModuleAddr, m.selfSlot(addr), u256ToHash(next))
	min := uint256.MustFromBig(m.cfg.MinSelfStake)
	if next.Cmp(min) < 0 {
		// drop candidate flag (keep list entry; ActiveSet filters by stake+flag)
		m.db.SetState(StakingModuleAddr, m.candSlot(addr), types.Hash{})
	}

	// Queue escrow: add to any existing pending; unlock = max(old, now+period).
	period := m.cfg.UnbondSeconds
	unlock := now + period
	pending, oldUnlock := m.PendingUnbond(addr)
	pending = new(uint256.Int).Add(pending, amount)
	if oldUnlock > unlock {
		unlock = oldUnlock
	}
	m.db.SetState(StakingModuleAddr, m.unbondAmtSlot(addr), u256ToHash(pending))
	m.db.SetState(StakingModuleAddr, m.unbondAtSlot(addr), u256ToHash(uint256.NewInt(unlock)))
	return nil
}

// Withdraw releases matured pending unbond to the caller credit amount.
// Returns the amount to transfer from module escrow; 0 error if not ready.
func (m *StakingModule) Withdraw(addr crypto.Address, now uint64) (*uint256.Int, error) {
	pending, unlockAt := m.PendingUnbond(addr)
	if pending == nil || pending.IsZero() {
		return nil, fmt.Errorf("staking: no pending unbond")
	}
	if now < unlockAt {
		return nil, fmt.Errorf("staking: unbonding period not elapsed (unlockAt=%d now=%d)", unlockAt, now)
	}
	m.db.SetState(StakingModuleAddr, m.unbondAmtSlot(addr), types.Hash{})
	m.db.SetState(StakingModuleAddr, m.unbondAtSlot(addr), types.Hash{})
	return pending, nil
}

// Jail marks a validator jailed without burning stake (legacy helper / tests).
// Production double-sign path uses SlashAndJail.
func (m *StakingModule) Jail(addr crypto.Address) {
	m.db.SetState(StakingModuleAddr, m.jailedSlot(addr), types.BytesToHash([]byte{1}))
	// candidate flag cleared so active set drops them immediately
	m.db.SetState(StakingModuleAddr, m.candSlot(addr), types.Hash{})
}

// SlashAndJail applies double-sign burn percentages then jails.
// Idempotent: if already jailed, returns zero burn and leaves state unchanged.
// Burned wei is subtracted from the module escrow balance (true burn).
func (m *StakingModule) SlashAndJail(addr crypto.Address) *uint256.Int {
	if m.IsJailed(addr) {
		return uint256.NewInt(0)
	}
	selfBps := params.DoubleSignSelfBurnBps
	delBps := params.DoubleSignDelegatorBurnBps
	if selfBps > MaxCommissionBps {
		selfBps = MaxCommissionBps
	}
	if delBps > MaxCommissionBps {
		delBps = MaxCommissionBps
	}

	self := m.SelfStake(addr)
	selfBurn := new(uint256.Int).Div(new(uint256.Int).Mul(self, uint256.NewInt(selfBps)), uint256.NewInt(MaxCommissionBps))
	if !selfBurn.IsZero() {
		nextSelf := new(uint256.Int).Sub(self, selfBurn)
		m.db.SetState(StakingModuleAddr, m.selfSlot(addr), u256ToHash(nextSelf))
	}

	delEff := m.DelegatedTotal(addr)
	delBurn := new(uint256.Int).Div(new(uint256.Int).Mul(delEff, uint256.NewInt(delBps)), uint256.NewInt(MaxCommissionBps))
	if !delBurn.IsZero() && !delEff.IsZero() {
		// rate' = rate * (10000 - delBps) / 10000
		rate := m.DelRate(addr)
		remainBps := MaxCommissionBps - delBps
		newRate := new(uint256.Int).Div(new(uint256.Int).Mul(rate, uint256.NewInt(remainBps)), uint256.NewInt(MaxCommissionBps))
		if newRate.IsZero() && remainBps > 0 {
			newRate = uint256.NewInt(1) // avoid zero rate trapping funds
		}
		m.db.SetState(StakingModuleAddr, m.delRateSlot(addr), u256ToHash(newRate))
	}

	totalBurn := new(uint256.Int).Add(selfBurn, delBurn)
	if !totalBurn.IsZero() {
		bal := m.db.GetBalance(StakingModuleAddr)
		if bal.Cmp(totalBurn) < 0 {
			// Fail-closed: burn only what the module holds (should not happen if escrow matches).
			totalBurn = new(uint256.Int).Set(bal)
		}
		if !totalBurn.IsZero() {
			m.db.SubBalance(StakingModuleAddr, totalBurn)
		}
	}

	m.Jail(addr)
	return totalBurn
}

// Delegate credits CALLVALUE amount (wei) from delegator as shares at current rate.
// Self-delegation is forbidden (use Bond for self-stake).
func (m *StakingModule) Delegate(validator, delegator crypto.Address, amount *uint256.Int) error {
	if amount == nil || amount.IsZero() {
		return fmt.Errorf("staking: zero delegate")
	}
	if validator == delegator {
		return fmt.Errorf("staking: self-delegate forbidden (use bond)")
	}
	m.settleRewards(validator, delegator)
	sharesAdd := m.weiToShares(validator, amount)
	if sharesAdd.IsZero() {
		return fmt.Errorf("staking: delegate amount too small for rate")
	}
	cur := m.DelegationShares(validator, delegator)
	next := new(uint256.Int).Add(cur, sharesAdd)
	m.db.SetState(StakingModuleAddr, m.delSlot(validator, delegator), u256ToHash(next))
	tot := new(uint256.Int).Add(m.DelegatedTotalShares(validator), sharesAdd)
	m.db.SetState(StakingModuleAddr, m.delTotSlot(validator), u256ToHash(tot))
	m.setRewardDebt(validator, delegator, m.rewardDebtForShares(validator, next))
	return nil
}

// Undelegate reduces live effective wei and queues that wei until now+UnbondSeconds.
func (m *StakingModule) Undelegate(validator, delegator crypto.Address, amount *uint256.Int, now uint64) error {
	if amount == nil || amount.IsZero() {
		return fmt.Errorf("staking: zero undelegate")
	}
	m.settleRewards(validator, delegator)
	curEff := m.Delegation(validator, delegator)
	if curEff.Cmp(amount) < 0 {
		return fmt.Errorf("staking: insufficient delegation")
	}
	sharesSub := m.weiToShares(validator, amount)
	if sharesSub.IsZero() {
		return fmt.Errorf("staking: undelegate amount too small for rate")
	}
	curShares := m.DelegationShares(validator, delegator)
	if curShares.Cmp(sharesSub) < 0 {
		sharesSub = new(uint256.Int).Set(curShares) // floor dust: burn remaining shares
	}
	nextShares := new(uint256.Int).Sub(curShares, sharesSub)
	m.db.SetState(StakingModuleAddr, m.delSlot(validator, delegator), u256ToHash(nextShares))
	totShares := m.DelegatedTotalShares(validator)
	if totShares.Cmp(sharesSub) < 0 {
		return fmt.Errorf("staking: delegated total underflow")
	}
	m.db.SetState(StakingModuleAddr, m.delTotSlot(validator), u256ToHash(new(uint256.Int).Sub(totShares, sharesSub)))
	m.setRewardDebt(validator, delegator, m.rewardDebtForShares(validator, nextShares))

	period := m.cfg.UnbondSeconds
	unlock := now + period
	pending, oldUnlock := m.PendingUndelegation(validator, delegator)
	pending = new(uint256.Int).Add(pending, amount)
	if oldUnlock > unlock {
		unlock = oldUnlock
	}
	m.db.SetState(StakingModuleAddr, m.delUnbondAmtSlot(validator, delegator), u256ToHash(pending))
	m.db.SetState(StakingModuleAddr, m.delUnbondAtSlot(validator, delegator), u256ToHash(uint256.NewInt(unlock)))
	return nil
}

// WithdrawDelegation releases matured pending undelegation for (validator, delegator).
func (m *StakingModule) WithdrawDelegation(validator, delegator crypto.Address, now uint64) (*uint256.Int, error) {
	pending, unlockAt := m.PendingUndelegation(validator, delegator)
	if pending == nil || pending.IsZero() {
		return nil, fmt.Errorf("staking: no pending undelegation")
	}
	if now < unlockAt {
		return nil, fmt.Errorf("staking: undelegation period not elapsed (unlockAt=%d now=%d)", unlockAt, now)
	}
	m.db.SetState(StakingModuleAddr, m.delUnbondAmtSlot(validator, delegator), types.Hash{})
	m.db.SetState(StakingModuleAddr, m.delUnbondAtSlot(validator, delegator), types.Hash{})
	return pending, nil
}

// SetCommission stores commission bps for validator (0–MaxCommissionBps).
func (m *StakingModule) SetCommission(validator crypto.Address, bps uint64) error {
	if bps > MaxCommissionBps {
		return fmt.Errorf("staking: commission bps > %d", MaxCommissionBps)
	}
	m.db.SetState(StakingModuleAddr, m.commSlot(validator), u256ToHash(uint256.NewInt(bps)))
	return nil
}

// RewardIndex returns cumulative reward-per-share × 1e18.
func (m *StakingModule) RewardIndex(validator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.rewIdxSlot(validator)))
}

func (m *StakingModule) rewardDebt(validator, delegator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.rewDebtSlot(validator, delegator)))
}

func (m *StakingModule) setRewardDebt(validator, delegator crypto.Address, debt *uint256.Int) {
	m.db.SetState(StakingModuleAddr, m.rewDebtSlot(validator, delegator), u256ToHash(debt))
}

func (m *StakingModule) pendingStored(validator, delegator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.rewPendSlot(validator, delegator)))
}

func (m *StakingModule) setPendingStored(validator, delegator crypto.Address, v *uint256.Int) {
	m.db.SetState(StakingModuleAddr, m.rewPendSlot(validator, delegator), u256ToHash(v))
}

// rewardDebtForShares = shares * idx / 1e18
func (m *StakingModule) rewardDebtForShares(validator crypto.Address, shares *uint256.Int) *uint256.Int {
	if shares == nil || shares.IsZero() {
		return uint256.NewInt(0)
	}
	idx := m.RewardIndex(validator)
	return new(uint256.Int).Div(new(uint256.Int).Mul(shares, idx), stakePrecision())
}

// settleRewards moves newly accrued index rewards into rewPend and refreshes debt.
func (m *StakingModule) settleRewards(validator, delegator crypto.Address) {
	shares := m.DelegationShares(validator, delegator)
	owed := m.rewardDebtForShares(validator, shares)
	debt := m.rewardDebt(validator, delegator)
	if owed.Cmp(debt) > 0 {
		delta := new(uint256.Int).Sub(owed, debt)
		pend := new(uint256.Int).Add(m.pendingStored(validator, delegator), delta)
		m.setPendingStored(validator, delegator, pend)
	}
	m.setRewardDebt(validator, delegator, owed)
}

// PendingRewards returns claimable rewards (settles index into pending first).
func (m *StakingModule) PendingRewards(validator, delegator crypto.Address) *uint256.Int {
	m.settleRewards(validator, delegator)
	return m.pendingStored(validator, delegator)
}

// ClaimRewards settles and returns wei to transfer from module to delegator; clears pending.
func (m *StakingModule) ClaimRewards(validator, delegator crypto.Address) (*uint256.Int, error) {
	m.settleRewards(validator, delegator)
	pend := m.pendingStored(validator, delegator)
	if pend == nil || pend.IsZero() {
		return nil, fmt.Errorf("staking: no rewards")
	}
	m.setPendingStored(validator, delegator, uint256.NewInt(0))
	return pend, nil
}

// IncreaseRewardIndex credits delPart wei into the per-share index (shares > 0).
// Caller must have already escrowed delPart at the module address.
func (m *StakingModule) IncreaseRewardIndex(validator crypto.Address, delPart, shares *uint256.Int) {
	if delPart == nil || delPart.IsZero() || shares == nil || shares.IsZero() {
		return
	}
	// idx += delPart * 1e18 / shares
	inc := new(uint256.Int).Div(new(uint256.Int).Mul(delPart, stakePrecision()), shares)
	if inc.IsZero() {
		return
	}
	next := new(uint256.Int).Add(m.RewardIndex(validator), inc)
	m.db.SetState(StakingModuleAddr, m.rewIdxSlot(validator), u256ToHash(next))
}

// DistributeProposerIncome splits tip/fee T when staking is enabled.
// Formula: V = T * (S*10000 + D*c) / (P*10000); R = T - V.
// V → proposer; R → module + reward index (or proposer if no shares).
func DistributeProposerIncome(db *state.StateDB, cfg StakingConfig, proposer crypto.Address, amount *uint256.Int, stakingEnabled bool) {
	if amount == nil || amount.IsZero() {
		return
	}
	if !stakingEnabled {
		db.AddBalancePrev(proposer, amount)
		return
	}
	mod := NewStakingModule(db, cfg)
	S := mod.SelfStake(proposer)
	D := mod.DelegatedTotal(proposer) // effective wei
	P := new(uint256.Int).Add(S, D)
	if P.IsZero() {
		db.AddBalancePrev(proposer, amount)
		return
	}
	c := mod.CommissionBps(proposer)
	// num = S*10000 + D*c ; den = P*10000
	num := new(uint256.Int).Add(
		new(uint256.Int).Mul(S, uint256.NewInt(MaxCommissionBps)),
		new(uint256.Int).Mul(D, uint256.NewInt(c)),
	)
	den := new(uint256.Int).Mul(P, uint256.NewInt(MaxCommissionBps))
	valGets := new(uint256.Int).Div(new(uint256.Int).Mul(amount, num), den)
	if valGets.Cmp(amount) > 0 {
		valGets = new(uint256.Int).Set(amount)
	}
	delPart := new(uint256.Int).Sub(amount, valGets)
	if !valGets.IsZero() {
		db.AddBalancePrev(proposer, valGets)
	}
	if delPart.IsZero() {
		return
	}
	shares := mod.DelegatedTotalShares(proposer)
	if shares.IsZero() {
		db.AddBalancePrev(proposer, delPart)
		return
	}
	// Escrow delegator pool at module; index accrues claim rights.
	db.AddBalancePrev(StakingModuleAddr, delPart)
	mod.IncreaseRewardIndex(proposer, delPart, shares)
}

// Candidate is one staked account entry for ranking.
type Candidate struct {
	Address     crypto.Address
	VotingPower *uint256.Int
}

// ActiveSet returns top-K candidates by voting power (deterministic address tie-break).
// Eligibility requires SelfStake ≥ min (not pure-delegation).
func (m *StakingModule) ActiveSet() []Candidate {
	n := hashToU256(m.db.GetState(StakingModuleAddr, m.lenSlot())).Uint64()
	var list []Candidate
	min := uint256.MustFromBig(m.cfg.MinSelfStake)
	for i := uint64(0); i < n; i++ {
		h := m.db.GetState(StakingModuleAddr, m.idxSlot(i))
		var addr crypto.Address
		copy(addr[:], h[12:])
		if addr == (crypto.Address{}) {
			continue
		}
		if !m.IsCandidate(addr) || m.IsJailed(addr) {
			continue
		}
		self := m.SelfStake(addr)
		if self.Cmp(min) < 0 {
			continue
		}
		vp := m.VotingPower(addr)
		if vp.IsZero() {
			continue
		}
		list = append(list, Candidate{Address: addr, VotingPower: vp})
	}
	sort.Slice(list, func(i, j int) bool {
		cmp := list[i].VotingPower.Cmp(list[j].VotingPower)
		if cmp != 0 {
			return cmp > 0 // higher power first
		}
		return bytesLess(list[i].Address[:], list[j].Address[:])
	})
	capN := m.cfg.ActiveCap
	if uint64(len(list)) > capN {
		list = list[:capN]
	}
	return list
}

func bytesLess(a, b []byte) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
