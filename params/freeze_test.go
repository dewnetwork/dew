package params

import "testing"

func TestPublicTestnetFreezeConsistency(t *testing.T) {
	if PublicTestnetChainID != 2026 {
		t.Fatalf("chain id %d want 2026", PublicTestnetChainID)
	}
	if PublicTestnetBlockGasLimit != DefaultBlockGasLimit {
		t.Fatalf("block gas limit %d != DefaultBlockGasLimit %d", PublicTestnetBlockGasLimit, DefaultBlockGasLimit)
	}
	if DefaultBlockGasLimit != 120_000_000 {
		t.Fatalf("DefaultBlockGasLimit %d", DefaultBlockGasLimit)
	}
	if PublicTestnetBaseFeeWei != 1_000_000_000 {
		t.Fatalf("base fee %d", PublicTestnetBaseFeeWei)
	}
	if MinDewTxFeeWei != DefaultDewTxFeeWei || DefaultDewTxFeeWei != 2_100_000_000_000 {
		t.Fatalf("DewTx fee floor drift: min=%d def=%d", MinDewTxFeeWei, DefaultDewTxFeeWei)
	}
	if NativeTransferPrecompileGas != 3_000 {
		t.Fatalf("0x100 gas %d", NativeTransferPrecompileGas)
	}
	if PrecompileNativeTransferAddr != 0x100 || PrecompileStakingAddr != 0x102 {
		t.Fatalf("precompile addresses 0x%x 0x%x", PrecompileNativeTransferAddr, PrecompileStakingAddr)
	}
	if DewTxVersion != 1 || DewTxDomainTag != "DewTx:v1" {
		t.Fatalf("DewTx wire: version=%d domain=%q", DewTxVersion, DewTxDomainTag)
	}
	if PublicTestnetStakingOn {
		t.Fatal("public testnet default must keep staking opt-in (off)")
	}
	if !PublicTestnetNativePathOn || !PublicTestnetPrecompilesOn {
		t.Fatal("native path and precompiles expected on for public testnet defaults")
	}
	// Staking module numbers frozen as public-testnet-v1 candidates.
	if DefaultEpochLengthBlocks != 86_400 {
		t.Fatalf("epoch length %d", DefaultEpochLengthBlocks)
	}
	if DefaultActiveValidatorCap != 100 {
		t.Fatalf("active cap %d", DefaultActiveValidatorCap)
	}
	if DefaultUnbondingPeriodSeconds != 604_800 {
		t.Fatalf("unbonding %d", DefaultUnbondingPeriodSeconds)
	}
	min := MinValidatorStakeWei()
	want := "100000000000000000000000" // 100_000 * 10^18
	if min.String() != want {
		t.Fatalf("min stake %s want %s", min.String(), want)
	}
}
