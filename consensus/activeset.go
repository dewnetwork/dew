package consensus

import (
	"fmt"
	"math/big"

	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/crypto"
)

// ActiveSetToValidatorSet maps staking module ActiveSet() into a BFT ValidatorSet.
// Voting power is truncated to uint64 (saturates at max uint64 if larger).
// Returns an error if the set is empty (keep previous genesis set instead).
func ActiveSetToValidatorSet(candidates []native.Candidate) (*ValidatorSet, error) {
	if len(candidates) == 0 {
		return nil, fmt.Errorf("consensus: empty active set")
	}
	vals := make([]Validator, 0, len(candidates))
	for _, c := range candidates {
		if c.Address == (crypto.Address{}) {
			continue
		}
		if c.VotingPower == nil || c.VotingPower.IsZero() {
			continue
		}
		power := powerFromU256(c.VotingPower.ToBig())
		if power == 0 {
			continue
		}
		vals = append(vals, Validator{Address: c.Address, Power: power})
	}
	return NewValidatorSet(vals)
}

func powerFromU256(v *big.Int) uint64 {
	if v == nil || v.Sign() <= 0 {
		return 0
	}
	if v.IsUint64() {
		return v.Uint64()
	}
	// Saturate — stake above 2^64-1 still counts as max power for BFT weights.
	return ^uint64(0)
}

// ShouldRotateEpoch reports whether committed height is an epoch boundary.
// height is the committed block number; epochLength from genesis (blocks).
// Rotation runs when height > 0 and height % epochLength == 0.
func ShouldRotateEpoch(height, epochLength uint64) bool {
	if epochLength == 0 || height == 0 {
		return false
	}
	return height%epochLength == 0
}
