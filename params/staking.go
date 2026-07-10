package params

import "math/big"

// Staking parameters — frozen candidates for public-testnet-v1 (Phase C6).
// Align with docs/consensus/validators.md and docs/development/public-testnet.md.
// Residual: unbonding not fully enforced; ActiveSet not wired into live BFT each epoch.

const (
	// StakingPrecompileGasBond is gas for bond / self-stake register.
	StakingPrecompileGasBond uint64 = 50_000
	// StakingPrecompileGasUnbond is gas for unbond.
	StakingPrecompileGasUnbond uint64 = 40_000
	// StakingPrecompileGasQuery is gas for view methods.
	StakingPrecompileGasQuery uint64 = 2_000
	// StakingPrecompileGasJail is gas for double-sign jail path.
	StakingPrecompileGasJail uint64 = 30_000

	// DefaultEpochLengthBlocks matches validators doc (public-testnet-v1).
	DefaultEpochLengthBlocks uint64 = 86_400
	// DefaultActiveValidatorCap is max active set size K (module default).
	// Sample genesis.json may use a smaller cap for local multi-validator nets.
	DefaultActiveValidatorCap uint64 = 100
	// DefaultUnbondingPeriodSeconds is 7 days (public-testnet-v1).
	DefaultUnbondingPeriodSeconds uint64 = 604_800

	// DefaultEnableStaking is off; public operators opt in explicitly.
	DefaultEnableStaking = false
)

// MinValidatorStakeWei is 100_000 DEW in wei (100000 * 10^18).
func MinValidatorStakeWei() *big.Int {
	// 100000e18
	v := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	return v.Mul(v, big.NewInt(100_000))
}
