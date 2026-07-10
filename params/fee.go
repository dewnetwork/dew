package params

// Phase B4 fee policy — values tuned against load tests in tests/load and
// documented in docs/execution/gas-and-fees.md.

const (
	// SimpleTransferGas is the intrinsic gas for a plain EVM value transfer.
	SimpleTransferGas uint64 = 21_000

	// ReferenceBaseFeeWei is the reference base fee used when quoting
	// DewTx flat fees as a fraction of a simple transfer (1 gwei).
	ReferenceBaseFeeWei uint64 = 1_000_000_000

	// DewTxFeeNumerator / DewTxFeeDenominator express the target ratio of
	// DefaultDewTxFeeWei to (SimpleTransferGas * ReferenceBaseFeeWei).
	// Default is 1/10 (10%).
	DewTxFeeNumerator   uint64 = 1
	DewTxFeeDenominator uint64 = 10

	// MinDewTxFeeWei is a spam floor for native txs (cannot undercut via Fee=0
	// override in protocol validation — executor substitutes Default when 0).
	// Kept equal to Default for Phase B; operators may raise via future genesis.
	MinDewTxFeeWei uint64 = DefaultDewTxFeeWei

	// DefaultBlockGasLimit matches genesis / docs (120M).
	DefaultBlockGasLimit uint64 = 120_000_000

	// EIP1559 elasticity defaults (Ethereum mainnet-compatible).
	EIP1559ElasticityMultiplier  uint64 = 2
	EIP1559BaseFeeChangeDenominator uint64 = 8
)

// SimpleTransferCostWei returns gas * baseFee for a simple transfer at baseFee.
func SimpleTransferCostWei(baseFeeWei uint64) uint64 {
	return SimpleTransferGas * baseFeeWei
}

// TargetDewTxFeeWei returns floor(simpleTransferCost * num / den) at the given base fee.
func TargetDewTxFeeWei(baseFeeWei uint64) uint64 {
	return SimpleTransferCostWei(baseFeeWei) * DewTxFeeNumerator / DewTxFeeDenominator
}
