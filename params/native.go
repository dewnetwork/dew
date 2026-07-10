// Package params holds chain constants with no heavy dependencies.
package params

// DewTxVersion is the frozen codec / signature domain version for Phase B.
const DewTxVersion uint32 = 1

// DewTxDomainTag is mixed into the signing hash so DewTx signatures
// cannot be replayed as EVM transactions (domain separation).
const DewTxDomainTag = "DewTx:v1"

// DefaultDewTxFeeWei is the flat fee for a native DewTx (wei).
// Directionally ~10% of a simple EVM transfer at 1 gwei base fee:
//
//	21_000 gas × 1e9 wei/gas × 0.10 = 2.1e12 wei
const DefaultDewTxFeeWei uint64 = 2_100_000_000_000

// Feature flags (runtime; genesis/config may override later).

// DefaultEnableNativePath enables dew_sendRawTransaction / DewTx execution.
const DefaultEnableNativePath = true

// DefaultEnableDewPrecompiles enables Dew system precompiles (0x100+).
const DefaultEnableDewPrecompiles = true

// NativeTransferPrecompileGas is the fixed gas for address 0x100.
const NativeTransferPrecompileGas uint64 = 3_000

// StakingPrecompileGas is the default query gas for address 0x102.
// Method-specific costs live in params/staking.go (bond/unbond/jail).
const StakingPrecompileGas uint64 = 2_000
