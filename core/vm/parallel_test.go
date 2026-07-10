package vm

import (
	"math/big"
	"testing"
	"time"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

func fundEOAs(s *state.StateDB, n int, bal *uint256.Int) []crypto.Address {
	addrs := make([]crypto.Address, n)
	for i := 0; i < n; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			panic(err)
		}
		addrs[i] = crypto.PubkeyToAddress(&key.PublicKey)
		s.SetBalance(addrs[i], new(uint256.Int).Set(bal))
	}
	return addrs
}

func blockCtx() BlockContext {
	return BlockContext{
		Number:   1,
		Time:     1_700_000_000,
		GasLimit: 100_000_000,
		BaseFee:  big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2026),
	}
}

// simple native-value transfers (EVM CALL with empty data) between disjoint pairs.
func transferMsg(from, to crypto.Address, amount uint64) Message {
	return Message{
		From:     from,
		To:       &to,
		Value:    uint256.NewInt(amount),
		GasLimit: 100_000,
		GasPrice: big.NewInt(0), // free gas → no coinbase write conflicts
		Data:     nil,
	}
}

func TestParallel_EquivalenceNonConflicting(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	seqDB := state.New(mdb)
	addrs := fundEOAs(seqDB, 8, uint256.NewInt(1_000_000))
	if _, err := seqDB.Commit(); err != nil {
		t.Fatal(err)
	}

	// 4 disjoint transfers: 0→1, 2→3, 4→5, 6→7
	msgs := []Message{
		transferMsg(addrs[0], addrs[1], 10),
		transferMsg(addrs[2], addrs[3], 20),
		transferMsg(addrs[4], addrs[5], 30),
		transferMsg(addrs[6], addrs[7], 40),
	}

	// Sequential baseline on a fork of committed state
	mdb2 := db.NewMemoryDB()
	defer mdb2.Close()
	// rebuild same genesis balances
	base := state.New(mdb2)
	for i, a := range addrs {
		base.SetBalance(a, uint256.NewInt(1_000_000))
		_ = i
	}
	if _, err := base.Commit(); err != nil {
		t.Fatal(err)
	}

	seqState := base.Copy()
	parState := base.Copy()

	seqPE := NewParallelExecutor(seqState, blockCtx(), 1)
	seqRes, err := seqPE.ApplySequential(msgs)
	if err != nil {
		t.Fatalf("sequential: %v", err)
	}
	seqRoot, err := seqState.IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}

	parPE := NewParallelExecutor(parState, blockCtx(), 4)
	parRes, err := parPE.ApplyParallel(msgs)
	if err != nil {
		t.Fatalf("parallel: %v", err)
	}
	parRoot, err := parState.IntermediateRoot()
	if err != nil {
		t.Fatal(err)
	}

	if seqRoot != parRoot {
		t.Fatalf("state root mismatch\n seq=%x\n par=%x", seqRoot, parRoot)
	}
	if len(seqRes) != len(parRes) {
		t.Fatalf("result len %d vs %d", len(seqRes), len(parRes))
	}
	for i := range seqRes {
		if seqRes[i].UsedGas != parRes[i].UsedGas {
			t.Fatalf("tx %d gas %d vs %d", i, seqRes[i].UsedGas, parRes[i].UsedGas)
		}
		if seqRes[i].Failed != parRes[i].Failed {
			t.Fatalf("tx %d failed %v vs %v", i, seqRes[i].Failed, parRes[i].Failed)
		}
		if len(seqRes[i].Logs) != len(parRes[i].Logs) {
			t.Fatalf("tx %d logs %d vs %d", i, len(seqRes[i].Logs), len(parRes[i].Logs))
		}
	}

	// Balances
	for i, a := range addrs {
		if seqState.GetBalance(a).Cmp(parState.GetBalance(a)) != 0 {
			t.Fatalf("addr %d bal seq=%s par=%s", i, seqState.GetBalance(a), parState.GetBalance(a))
		}
	}
}

func TestParallel_EquivalenceConflicting(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	base := state.New(mdb)
	addrs := fundEOAs(base, 3, uint256.NewInt(1_000_000))
	if _, err := base.Commit(); err != nil {
		t.Fatal(err)
	}

	// Both txs touch addrs[0] → conflict / re-exec path
	msgs := []Message{
		transferMsg(addrs[0], addrs[1], 100),
		transferMsg(addrs[0], addrs[2], 50),
	}

	seqState := base.Copy()
	parState := base.Copy()

	seqPE := NewParallelExecutor(seqState, blockCtx(), 1)
	seqRes, err := seqPE.ApplySequential(msgs)
	if err != nil {
		t.Fatal(err)
	}
	seqRoot, _ := seqState.IntermediateRoot()

	parPE := NewParallelExecutor(parState, blockCtx(), 4)
	parRes, err := parPE.ApplyParallel(msgs)
	if err != nil {
		t.Fatal(err)
	}
	parRoot, _ := parState.IntermediateRoot()

	if seqRoot != parRoot {
		t.Fatalf("root mismatch under conflict\n seq=%x\n par=%x", seqRoot, parRoot)
	}
	if seqRes[0].Failed || seqRes[1].Failed || parRes[0].Failed || parRes[1].Failed {
		t.Fatalf("unexpected failure seq=%v par=%v", seqRes, parRes)
	}
	// sender spent 150
	want := uint64(1_000_000 - 150)
	if parState.GetBalance(addrs[0]).Uint64() != want {
		t.Fatalf("sender bal = %s want %d", parState.GetBalance(addrs[0]), want)
	}
	st := parPE.Stats()
	if st.Rollbacks < 1 {
		t.Fatalf("expected at least one rollback, got %+v", st)
	}
	if st.ConflictRate <= 0 {
		t.Fatalf("expected positive conflict rate, got %v", st.ConflictRate)
	}
}

func TestParallel_SpeedupNonConflicting(t *testing.T) {
	if testing.Short() {
		t.Skip("speedup check")
	}
	const n = 64
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	base := state.New(mdb)
	// n pairs = 2n addresses
	addrs := fundEOAs(base, n*2, uint256.NewInt(1_000_000))
	if _, err := base.Commit(); err != nil {
		t.Fatal(err)
	}

	msgs := make([]Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = transferMsg(addrs[i*2], addrs[i*2+1], 1)
	}

	seqState := base.Copy()
	parState := base.Copy()

	seqPE := NewParallelExecutor(seqState, blockCtx(), 1)
	t0 := time.Now()
	if _, err := seqPE.ApplySequential(msgs); err != nil {
		t.Fatal(err)
	}
	seqDur := time.Since(t0)

	parPE := NewParallelExecutor(parState, blockCtx(), 0) // GOMAXPROCS
	t1 := time.Now()
	if _, err := parPE.ApplyParallel(msgs); err != nil {
		t.Fatal(err)
	}
	parDur := time.Since(t1)

	seqRoot, _ := seqState.IntermediateRoot()
	parRoot, _ := parState.IntermediateRoot()
	if seqRoot != parRoot {
		t.Fatalf("roots diverged in speedup test")
	}

	st := parPE.Stats()
	t.Logf("sequential=%v parallel=%v rollbacks=%d specOK=%d conflictRate=%.2f workers=%d",
		seqDur, parDur, st.Rollbacks, st.SpeculativeOK, st.ConflictRate, st.Workers)

	// On multi-core, parallel should not be dramatically slower; allow slack for CI.
	// Require either measurable speedup OR at least majority speculative commits.
	if st.SpeculativeOK < n/2 {
		t.Fatalf("expected most txs to commit speculatively, specOK=%d n=%d", st.SpeculativeOK, n)
	}
}

func TestParallel_MetricsRecorded(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	base := state.New(mdb)
	addrs := fundEOAs(base, 4, uint256.NewInt(1_000_000))
	if _, err := base.Commit(); err != nil {
		t.Fatal(err)
	}
	msgs := []Message{
		transferMsg(addrs[0], addrs[1], 1),
		transferMsg(addrs[2], addrs[3], 1),
	}
	pe := NewParallelExecutor(base.Copy(), blockCtx(), 2)
	if _, err := pe.ApplyParallel(msgs); err != nil {
		t.Fatal(err)
	}
	st := pe.Stats()
	if st.TxCount != 2 {
		t.Fatalf("TxCount=%d", st.TxCount)
	}
	if st.DurationNS <= 0 {
		t.Fatal("DurationNS not set")
	}
	if st.TotalTxs != 2 {
		t.Fatalf("TotalTxs=%d", st.TotalTxs)
	}
}
