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
//	keccak256("dew/stake/v1/del" || val || del)  → live delegation amount
//	keccak256("dew/stake/v1/delTot" || val)      → total delegated to validator
//	keccak256("dew/stake/v1/comm" || val)        → commission bps (0–10000)
//	keccak256("dew/stake/v1/delUnbondAmt" || val || del) → pending undelegation
//	keccak256("dew/stake/v1/delUnbondAt" || val || del)  → unlock unix
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

// Delegation returns live delegated amount from delegator to validator.
func (m *StakingModule) Delegation(validator, delegator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.delSlot(validator, delegator)))
}

// DelegatedTotal returns sum of live delegations to validator.
func (m *StakingModule) DelegatedTotal(validator crypto.Address) *uint256.Int {
	return hashToU256(m.db.GetState(StakingModuleAddr, m.delTotSlot(validator)))
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

// Jail marks a validator jailed (double-sign / evidence path). Voting power becomes 0.
// Fail closed: no-op error if already jailed; always sets jail bit when called.
func (m *StakingModule) Jail(addr crypto.Address) {
	m.db.SetState(StakingModuleAddr, m.jailedSlot(addr), types.BytesToHash([]byte{1}))
	// candidate flag cleared so active set drops them immediately
	m.db.SetState(StakingModuleAddr, m.candSlot(addr), types.Hash{})
}

// Delegate credits CALLVALUE amount from delegator to validator's delegated total.
// Self-delegation is forbidden (use Bond for self-stake).
func (m *StakingModule) Delegate(validator, delegator crypto.Address, amount *uint256.Int) error {
	if amount == nil || amount.IsZero() {
		return fmt.Errorf("staking: zero delegate")
	}
	if validator == delegator {
		return fmt.Errorf("staking: self-delegate forbidden (use bond)")
	}
	cur := m.Delegation(validator, delegator)
	next := new(uint256.Int).Add(cur, amount)
	m.db.SetState(StakingModuleAddr, m.delSlot(validator, delegator), u256ToHash(next))
	tot := new(uint256.Int).Add(m.DelegatedTotal(validator), amount)
	m.db.SetState(StakingModuleAddr, m.delTotSlot(validator), u256ToHash(tot))
	return nil
}

// Undelegate reduces live delegation and queues amount until now+UnbondSeconds.
func (m *StakingModule) Undelegate(validator, delegator crypto.Address, amount *uint256.Int, now uint64) error {
	if amount == nil || amount.IsZero() {
		return fmt.Errorf("staking: zero undelegate")
	}
	cur := m.Delegation(validator, delegator)
	if cur.Cmp(amount) < 0 {
		return fmt.Errorf("staking: insufficient delegation")
	}
	next := new(uint256.Int).Sub(cur, amount)
	m.db.SetState(StakingModuleAddr, m.delSlot(validator, delegator), u256ToHash(next))
	tot := m.DelegatedTotal(validator)
	if tot.Cmp(amount) < 0 {
		return fmt.Errorf("staking: delegated total underflow")
	}
	m.db.SetState(StakingModuleAddr, m.delTotSlot(validator), u256ToHash(new(uint256.Int).Sub(tot, amount)))

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
