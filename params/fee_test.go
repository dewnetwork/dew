package params

import "testing"

func TestDewTxFee_MatchesTenPercentOfSimpleTransfer(t *testing.T) {
	want := TargetDewTxFeeWei(ReferenceBaseFeeWei)
	if DefaultDewTxFeeWei != want {
		t.Fatalf("DefaultDewTxFeeWei=%d want Target=%d (10%% of 21k*1gwei)", DefaultDewTxFeeWei, want)
	}
	if DefaultDewTxFeeWei != 2_100_000_000_000 {
		t.Fatalf("unexpected constant %d", DefaultDewTxFeeWei)
	}
	if MinDewTxFeeWei > DefaultDewTxFeeWei {
		t.Fatal("min fee must not exceed default")
	}
	// Precompile gas << simple transfer
	if NativeTransferPrecompileGas >= SimpleTransferGas {
		t.Fatalf("0x100 gas %d should be cheaper than simple transfer %d",
			NativeTransferPrecompileGas, SimpleTransferGas)
	}
	// Orderbook reserved gas budget sanity for stub
	if StakingPrecompileGas == 0 || StakingPrecompileGas > 100_000 {
		t.Fatalf("staking stub gas out of band: %d", StakingPrecompileGas)
	}
}

func TestTargetDewTxFee_ScalesWithBaseFee(t *testing.T) {
	at2gwei := TargetDewTxFeeWei(2_000_000_000)
	if at2gwei != 4_200_000_000_000 {
		t.Fatalf("at 2 gwei got %d", at2gwei)
	}
}
