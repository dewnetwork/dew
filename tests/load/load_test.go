// Package load holds Phase B4 in-process load / stress tests.
// Run: go test ./tests/load/ -count=1 -timeout 120s
package load

import (
	"fmt"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

func TestLoad_ParallelVsSequential_NonConflicting(t *testing.T) {
	const n = 128
	msgs, base := makeDisjointTransfers(t, n)

	seqState := base.Copy()
	parState := base.Copy()

	seqPE := vm.NewParallelExecutor(seqState, loadBlock(), 1)
	t0 := time.Now()
	seqRes, err := seqPE.ApplySequential(msgs)
	seqDur := time.Since(t0)
	if err != nil {
		t.Fatal(err)
	}

	parPE := vm.NewParallelExecutor(parState, loadBlock(), runtime.GOMAXPROCS(0))
	t1 := time.Now()
	parRes, err := parPE.ApplyParallel(msgs)
	parDur := time.Since(t1)
	if err != nil {
		t.Fatal(err)
	}

	seqRoot, err := seqState.IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}
	parRoot, err := parState.IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}
	if seqRoot != parRoot {
		t.Fatalf("roots diverged under load\nseq=%x\npar=%x", seqRoot, parRoot)
	}
	if len(seqRes) != len(parRes) {
		t.Fatalf("result count %d vs %d", len(seqRes), len(parRes))
	}

	st := parPE.Stats()
	t.Logf("n=%d workers=%d sequential=%v parallel=%v speedup=%.2fx rollbacks=%d conflict_rate=%.3f",
		n, st.Workers, seqDur, parDur, float64(seqDur)/float64(parDur), st.Rollbacks, st.ConflictRate)

	if st.SpeculativeOK < n/2 {
		t.Fatalf("expected majority speculative commits, specOK=%d", st.SpeculativeOK)
	}
	// Note: for tiny EVM transfers, fork+overlay overhead can exceed sequential
	// wall time; correctness + speculative commit rate are the hard gates.
	// Speedup is expected on heavier/non-conflicting contract work (see benches).
	if parDur > seqDur*10 {
		t.Fatalf("parallel pathologically slow: par=%v seq=%v", parDur, seqDur)
	}
}

func TestLoad_Parallel_MixedConflicts_Equivalence(t *testing.T) {
	const n = 64
	mdb := db.OpenTest(t)
	base := state.New(mdb)

	// Shared hub + many leaf pairs → partial conflicts
	hubKey, _ := crypto.GenerateKey()
	hub := crypto.PubkeyToAddress(&hubKey.PublicKey)
	base.SetBalance(hub, uint256.NewInt(10_000_000))

	msgs := make([]vm.Message, 0, n)
	for i := 0; i < n; i++ {
		k, _ := crypto.GenerateKey()
		a := crypto.PubkeyToAddress(&k.PublicKey)
		base.SetBalance(a, uint256.NewInt(1_000_000))
		if i%4 == 0 {
			// conflict path: debit hub
			to := a
			msgs = append(msgs, vm.Message{
				From: hub, To: &to, Value: uint256.NewInt(1),
				GasLimit: 100_000, GasPrice: big.NewInt(0),
			})
		} else {
			// disjoint: a → fresh
			k2, _ := crypto.GenerateKey()
			b := crypto.PubkeyToAddress(&k2.PublicKey)
			base.SetBalance(b, uint256.NewInt(0))
			msgs = append(msgs, vm.Message{
				From: a, To: &b, Value: uint256.NewInt(1),
				GasLimit: 100_000, GasPrice: big.NewInt(0),
			})
		}
	}
	if _, err := base.Commit(); err != nil {
		t.Fatal(err)
	}

	seqState := base.Copy()
	parState := base.Copy()
	if _, err := vm.NewParallelExecutor(seqState, loadBlock(), 1).ApplySequential(msgs); err != nil {
		t.Fatal(err)
	}
	pe := vm.NewParallelExecutor(parState, loadBlock(), 0)
	if _, err := pe.ApplyParallel(msgs); err != nil {
		t.Fatal(err)
	}
	sr, _ := seqState.IntermediateRoot()
	pr, _ := parState.IntermediateRoot()
	if sr != pr {
		t.Fatalf("mixed-conflict roots differ\nseq=%x\npar=%x", sr, pr)
	}
	st := pe.Stats()
	if st.Rollbacks == 0 {
		t.Log("warning: expected some rollbacks on mixed workload (hub txs)")
	}
	t.Logf("mixed n=%d rollbacks=%d conflict_rate=%.3f", n, st.Rollbacks, st.ConflictRate)
}

// TestLoad_PE_Matrix_ConflictAndWorkers is the S2 PE measurement matrix:
// sequential vs PE across conflict structures and worker counts.
// Hard gates: serial-equivalent roots. Wall-clock is logged, not required to speedup.
func TestLoad_PE_Matrix_ConflictAndWorkers(t *testing.T) {
	const n = 64
	workersList := []int{1, 2, 4, runtime.GOMAXPROCS(0)}
	// conflictEvery: 0 = all disjoint; k = every k-th tx debits a shared hub.
	scenarios := []struct {
		name          string
		conflictEvery int
	}{
		{name: "disjoint", conflictEvery: 0},
		{name: "mixed_1in4", conflictEvery: 4},
		{name: "hub_all", conflictEvery: 1},
	}

	t.Logf("matrix n=%d gomaxprocs=%d columns=scenario,workers,seq_ms,par_ms,speedup,rollbacks,conflict_rate,spec_ok,root_ok",
		n, runtime.GOMAXPROCS(0))

	for _, sc := range scenarios {
		sc := sc
		msgs, base := makeConflictWorkload(t, n, sc.conflictEvery)

		// Sequential baseline once per scenario (shared reference root).
		seqState := base.Copy()
		t0 := time.Now()
		if _, err := vm.NewParallelExecutor(seqState, loadBlock(), 1).ApplySequential(msgs); err != nil {
			t.Fatalf("%s sequential: %v", sc.name, err)
		}
		seqDur := time.Since(t0)
		seqRoot, err := seqState.IntermediateRoot()
		if err != nil {
			t.Fatal(err)
		}

		for _, w := range workersList {
			w := w
			parState := base.Copy()
			pe := vm.NewParallelExecutor(parState, loadBlock(), w)
			t1 := time.Now()
			if _, err := pe.ApplyParallel(msgs); err != nil {
				t.Fatalf("%s workers=%d parallel: %v", sc.name, w, err)
			}
			parDur := time.Since(t1)
			parRoot, err := parState.IntermediateRoot()
			if err != nil {
				t.Fatal(err)
			}
			rootOK := seqRoot == parRoot
			if !rootOK {
				t.Fatalf("%s workers=%d roots diverged\nseq=%x\npar=%x", sc.name, w, seqRoot, parRoot)
			}
			st := pe.Stats()
			speedup := float64(seqDur) / float64(parDur)
			t.Logf("ROW scenario=%s workers=%d seq_ms=%.3f par_ms=%.3f speedup=%.2f rollbacks=%d conflict_rate=%.3f spec_ok=%d root_ok=%v",
				sc.name, st.Workers,
				float64(seqDur.Microseconds())/1000.0,
				float64(parDur.Microseconds())/1000.0,
				speedup, st.Rollbacks, st.ConflictRate, st.SpeculativeOK, rootOK)

			// Pathological slow PE only (allow large slack for fork+overlay on tiny transfers).
			if parDur > seqDur*20 {
				t.Fatalf("%s workers=%d pathologically slow: par=%v seq=%v", sc.name, w, parDur, seqDur)
			}
		}
	}
}

func TestLoad_NativeDewTx_Throughput(t *testing.T) {
	const n = 200
	mdb := db.OpenTest(t)
	st := state.New(mdb)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	sink := crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0")
	need := uint64(n)*params.DefaultDewTxFeeWei + uint64(n)*10 + 1
	st.SetBalance(sender, uint256.NewInt(need))

	start := time.Now()
	for i := 0; i < n; i++ {
		tx := types.NewDewTx(big.NewInt(2205), uint64(i), sender, recv, uint256.NewInt(10), params.DefaultDewTxFeeWei, nil, nil)
		if err := types.SignDewTx(tx, key); err != nil {
			t.Fatal(err)
		}
		res, err := native.NewExecutor(st, sink).ApplyDewTx(tx)
		if err != nil || res.Failed {
			t.Fatalf("i=%d err=%v res=%+v", i, err, res)
		}
	}
	elapsed := time.Since(start)
	tps := float64(n) / elapsed.Seconds()
	t.Logf("native DewTx n=%d elapsed=%v tps=%.0f fee_wei=%d", n, elapsed, tps, params.DefaultDewTxFeeWei)
	if st.GetBalance(recv).Uint64() != uint64(n)*10 {
		t.Fatalf("recv bal=%s", st.GetBalance(recv))
	}
	if tps < 50 {
		// Extremely low bar — catches accidental catastrophic regressions only.
		t.Fatalf("native throughput too low: %.1f tps", tps)
	}
}

func TestLoad_ReportFeeComparison(t *testing.T) {
	// Documents cost ratio used for fee tuning (not a wall-clock test).
	evmTransferGas := uint64(21_000)
	baseFeeGwei := uint64(1) // 1 gwei
	evmCostWei := evmTransferGas * baseFeeGwei * 1_000_000_000
	nativeFee := params.DefaultDewTxFeeWei
	ratio := float64(nativeFee) / float64(evmCostWei)
	t.Logf("evm_simple_transfer_wei=%d dewtx_flat_fee_wei=%d ratio=%.3f target≈0.10",
		evmCostWei, nativeFee, ratio)
	if ratio < 0.05 || ratio > 0.20 {
		t.Fatalf("DewTx fee ratio %.3f outside tuning band [0.05, 0.20] of simple EVM transfer at 1 gwei", ratio)
	}
	// Precompile should be cheaper gas than a full ERC-20 transfer (~50k+)
	if params.NativeTransferPrecompileGas >= 21_000 {
		t.Fatalf("0x100 gas %d should be < intrinsic transfer", params.NativeTransferPrecompileGas)
	}
	_ = fmt.Sprintf
}

func makeDisjointTransfers(t *testing.T, n int) ([]vm.Message, *state.StateDB) {
	t.Helper()
	return makeConflictWorkload(t, n, 0)
}

// makeConflictWorkload builds n simple value-transfer messages.
// conflictEvery == 0: all pairs are disjoint (no shared accounts).
// conflictEvery == k (k > 0): every k-th tx debits a shared hub (and leaves are funded).
func makeConflictWorkload(t *testing.T, n, conflictEvery int) ([]vm.Message, *state.StateDB) {
	t.Helper()
	mdb := db.OpenTest(t)
	base := state.New(mdb)

	var hub crypto.Address
	if conflictEvery > 0 {
		hubKey, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		hub = crypto.PubkeyToAddress(&hubKey.PublicKey)
		// Enough balance for all hub debits (worst case every tx).
		base.SetBalance(hub, uint256.NewInt(uint64(n)+1_000_000))
	}

	msgs := make([]vm.Message, n)
	for i := 0; i < n; i++ {
		k, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		a := crypto.PubkeyToAddress(&k.PublicKey)
		base.SetBalance(a, uint256.NewInt(1_000_000))

		useHub := conflictEvery > 0 && i%conflictEvery == 0
		if useHub {
			to := a
			msgs[i] = vm.Message{
				From: hub, To: &to, Value: uint256.NewInt(1),
				GasLimit: 100_000, GasPrice: big.NewInt(0),
			}
			continue
		}
		k2, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		b := crypto.PubkeyToAddress(&k2.PublicKey)
		base.SetBalance(b, uint256.NewInt(0))
		to := b
		msgs[i] = vm.Message{
			From: a, To: &to, Value: uint256.NewInt(1),
			GasLimit: 100_000, GasPrice: big.NewInt(0),
		}
	}
	if _, err := base.Commit(); err != nil {
		t.Fatal(err)
	}
	return msgs, base
}

func loadBlock() vm.BlockContext {
	return vm.BlockContext{
		Number:   1,
		Time:     1_700_000_000,
		GasLimit: 120_000_000,
		BaseFee:  big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2205),
	}
}
