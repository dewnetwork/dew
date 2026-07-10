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
//	keccak256("dew/stake/v1/self" || addr)     → self-stake amount (32-byte big-endian)
//	keccak256("dew/stake/v1/jailed" || addr)   → 1 if jailed
//	keccak256("dew/stake/v1/cand" || addr)     → 1 if candidate registered
//	keccak256("dew/stake/v1/candlen")          → candidate count
//	keccak256("dew/stake/v1/candi" || uint64)  → candidate address at index
const (
	stakeDomainSelf   = "dew/stake/v1/self"
	stakeDomainJailed = "dew/stake/v1/jailed"
	stakeDomainCand   = "dew/stake/v1/cand"
	stakeDomainLen    = "dew/stake/v1/candlen"
	stakeDomainIdx    = "dew/stake/v1/candi"
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

// VotingPower is self-stake if candidate and not jailed, else 0.
func (m *StakingModule) VotingPower(addr crypto.Address) *uint256.Int {
	if m.IsJailed(addr) || !m.IsCandidate(addr) {
		return uint256.NewInt(0)
	}
	return m.SelfStake(addr)
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

// Unbond reduces self-stake and returns amount to credit back to addr.
// C4: funds return immediately; unbonding period enforcement is residual debt.
func (m *StakingModule) Unbond(addr crypto.Address, amount *uint256.Int) (*uint256.Int, error) {
	if amount == nil || amount.IsZero() {
		return nil, fmt.Errorf("staking: zero unbond")
	}
	if m.IsJailed(addr) {
		return nil, fmt.Errorf("staking: jailed")
	}
	cur := m.SelfStake(addr)
	if cur.Cmp(amount) < 0 {
		return nil, fmt.Errorf("staking: insufficient stake")
	}
	next := new(uint256.Int).Sub(cur, amount)
	m.db.SetState(StakingModuleAddr, m.selfSlot(addr), u256ToHash(next))
	min := uint256.MustFromBig(m.cfg.MinSelfStake)
	if next.Cmp(min) < 0 {
		// drop candidate flag (keep list entry; ActiveSet filters by stake+flag)
		m.db.SetState(StakingModuleAddr, m.candSlot(addr), types.Hash{})
	}
	return amount, nil
}

// Jail marks a validator jailed (double-sign / evidence path). Voting power becomes 0.
// Fail closed: no-op error if already jailed; always sets jail bit when called.
func (m *StakingModule) Jail(addr crypto.Address) {
	m.db.SetState(StakingModuleAddr, m.jailedSlot(addr), types.BytesToHash([]byte{1}))
	// candidate flag cleared so active set drops them immediately
	m.db.SetState(StakingModuleAddr, m.candSlot(addr), types.Hash{})
}

// Candidate is one staked account entry for ranking.
type Candidate struct {
	Address     crypto.Address
	VotingPower *uint256.Int
}

// ActiveSet returns top-K candidates by voting power (deterministic address tie-break).
func (m *StakingModule) ActiveSet() []Candidate {
	n := hashToU256(m.db.GetState(StakingModuleAddr, m.lenSlot())).Uint64()
	var list []Candidate
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
		vp := m.VotingPower(addr)
		if vp.IsZero() {
			continue
		}
		min := uint256.MustFromBig(m.cfg.MinSelfStake)
		if vp.Cmp(min) < 0 {
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
