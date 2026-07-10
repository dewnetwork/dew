package params

import "math/big"

// Staking parameters (tentative until C6 freeze). Align with docs/consensus/validators.md.

const (
	// StakingPrecompileGasBond is gas for bond / self-stake register.
	StakingPrecompileGasBond uint64 = 50_000
	// StakingPrecompileGasUnbond is gas for unbond.
	StakingPrecompileGasUnbond uint64 = 40_000
	// StakingPrecompileGasQuery is gas for view methods.
	StakingPrecompileGasQuery uint64 = 2_000
	// StakingPrecompileGasJail is gas for double-sign jail path.
	StakingPrecompileGasJail uint64 = 30_000

	// DefaultEpochLengthBlocks matches genesis / validators doc (_tentative_).
	DefaultEpochLengthBlocks uint64 = 86_400
	// DefaultActiveValidatorCap is max active set size K (_tentative_).
	DefaultActiveValidatorCap uint64 = 100
	// DefaultUnbondingPeriodSeconds is 7 days (_tentative_).
	DefaultUnbondingPeriodSeconds uint64 = 604_800

	// DefaultEnableStaking is off until private testnet operators opt in (C4).
	DefaultEnableStaking = false
)

// MinValidatorStakeWei is 100_000 DEW in wei (100000 * 10^18).
func MinValidatorStakeWei() *big.Int {
	// 100000e18
	v := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	return v.Mul(v, big.NewInt(100_000))
}
