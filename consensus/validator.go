package consensus

import (
	"fmt"
	"sort"

	"github.com/dewnetwork/dew/crypto"
)

// Validator is one bonded authority with stake-weighted voting power.
type Validator struct {
	Address crypto.Address
	Power   uint64
}

// ValidatorSet is the active set for an epoch (ordered deterministically by address).
type ValidatorSet struct {
	Validators []Validator
	totalPower uint64
	index      map[crypto.Address]int
}

// NewValidatorSet builds a set. Validators are sorted by address for stable
// proposer selection across nodes. Zero-power and duplicate addresses are rejected.
func NewValidatorSet(vals []Validator) (*ValidatorSet, error) {
	if len(vals) == 0 {
		return nil, fmt.Errorf("consensus: empty validator set")
	}
	// Copy and sort by address bytes.
	cp := make([]Validator, len(vals))
	copy(cp, vals)
	sort.Slice(cp, func(i, j int) bool {
		return string(cp[i].Address[:]) < string(cp[j].Address[:])
	})

	idx := make(map[crypto.Address]int, len(cp))
	var total uint64
	for i, v := range cp {
		if v.Power == 0 {
			return nil, fmt.Errorf("consensus: validator %s has zero power", v.Address.Hex())
		}
		if _, ok := idx[v.Address]; ok {
			return nil, fmt.Errorf("consensus: duplicate validator %s", v.Address.Hex())
		}
		idx[v.Address] = i
		// Saturating add would hide misconfig; reject overflow.
		if total+v.Power < total {
			return nil, fmt.Errorf("consensus: voting power overflow")
		}
		total += v.Power
	}
	return &ValidatorSet{Validators: cp, totalPower: total, index: idx}, nil
}

// Size returns the number of validators.
func (vs *ValidatorSet) Size() int { return len(vs.Validators) }

// TotalPower returns the sum of voting power.
func (vs *ValidatorSet) TotalPower() uint64 { return vs.totalPower }

// Get returns the validator and true if address is in the set.
func (vs *ValidatorSet) Get(addr crypto.Address) (Validator, bool) {
	i, ok := vs.index[addr]
	if !ok {
		return Validator{}, false
	}
	return vs.Validators[i], true
}

// PowerOf returns voting power for addr, or 0 if unknown.
func (vs *ValidatorSet) PowerOf(addr crypto.Address) uint64 {
	v, ok := vs.Get(addr)
	if !ok {
		return 0
	}
	return v.Power
}

// HasQuorum reports whether votePower is a strict supermajority: power * 3 > total * 2.
func (vs *ValidatorSet) HasQuorum(votePower uint64) bool {
	return HasQuorum(votePower, vs.totalPower)
}

// HasQuorum is the pure quorum check used by the engine and tests.
func HasQuorum(votePower, totalPower uint64) bool {
	if totalPower == 0 {
		return false
	}
	// Strict > 2/3 without floats: 3*vote > 2*total
	return votePower*3 > totalPower*2
}

// ProposerIndex returns the deterministic proposer for (height, round).
//
// Stake-weighted round-robin: slot = (height + round) mod TotalPower, then the
// first validator whose cumulative power exceeds the slot. Higher-power
// validators propose proportionally more often; selection is identical on all
// honest nodes given the same set.
func (vs *ValidatorSet) ProposerIndex(height, round uint64) int {
	total := vs.totalPower
	if total == 0 {
		return 0
	}
	slot := (height + round) % total
	var acc uint64
	for i, v := range vs.Validators {
		acc += v.Power
		if slot < acc {
			return i
		}
	}
	return len(vs.Validators) - 1
}

// Proposer returns the proposer address for (height, round).
func (vs *ValidatorSet) Proposer(height, round uint64) crypto.Address {
	return vs.Validators[vs.ProposerIndex(height, round)].Address
}
