package params

import "math/big"

// Staking parameters — frozen candidates for public-testnet-v1 (Phase C6).
// Align with docs/consensus/validators.md and docs/ops/public-testnet.md.
// Delegation + commission + tip/reward split + double-sign slash burn: D3c 2026-07-15.
// Nested zero-value unbond/withdraw/undelegate/claim = fail-closed tx.origin (S4 documented).
// Slash bps and reward split are **testnet provisional** (tokenomics still draft for mainnet).

const (
	// StakingPrecompileGasBond is gas for bond / self-stake register / delegate.
	StakingPrecompileGasBond uint64 = 50_000
	// StakingPrecompileGasUnbond is gas for unbond / undelegate / withdraw / claim paths.
	StakingPrecompileGasUnbond uint64 = 40_000
	// StakingPrecompileGasQuery is gas for view methods.
	StakingPrecompileGasQuery uint64 = 2_000
	// StakingPrecompileGasJail is gas for double-sign jail + slash path.
	StakingPrecompileGasJail uint64 = 30_000
	// StakingPrecompileGasSetCommission is gas for commission updates.
	StakingPrecompileGasSetCommission uint64 = 30_000
	// StakingPrecompileGasClaimRewards is gas for claimRewards.
	StakingPrecompileGasClaimRewards uint64 = 40_000

	// DefaultEpochLengthBlocks matches validators doc (public-testnet-v1).
	DefaultEpochLengthBlocks uint64 = 86_400
	// DefaultActiveValidatorCap is max active set size K (module default).
	// Sample genesis.json may use a smaller cap for local multi-validator nets.
	DefaultActiveValidatorCap uint64 = 100
	// DefaultUnbondingPeriodSeconds is 7 days (public-testnet-v1).
	DefaultUnbondingPeriodSeconds uint64 = 604_800

	// DefaultEnableStaking is off; public operators opt in explicitly.
	DefaultEnableStaking = false

	// DoubleSignSelfBurnBps burns this fraction of validator self-stake on double-sign (100% = 10000).
	// Provisional from docs/consensus/slashing.md; mainnet may re-freeze.
	DoubleSignSelfBurnBps uint64 = 10_000
	// DoubleSignDelegatorBurnBps burns this fraction of effective delegated stake (5% = 500).
	DoubleSignDelegatorBurnBps uint64 = 500

	// StakeRatePrecision is 1e18 — del exchange rate and reward-index scale.
	StakeRatePrecision uint64 = 1_000_000_000_000_000_000
)

// MinValidatorStakeWei is 100_000 DEW in wei (100000 * 10^18).
func MinValidatorStakeWei() *big.Int {
	// 100000e18
	v := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	return v.Mul(v, big.NewInt(100_000))
}
