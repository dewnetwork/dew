package vm

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

// BenchmarkSequentialTransfers measures sequential EVM value transfers.
func BenchmarkSequentialTransfers(b *testing.B) {
	const n = 64
	msgs, base := benchTransferFixture(b, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		st := base.Copy()
		pe := NewParallelExecutor(st, blockCtx(), 1)
		b.StartTimer()
		if _, err := pe.ApplySequential(msgs); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelTransfers measures Dew-PE on non-conflicting transfers.
func BenchmarkParallelTransfers(b *testing.B) {
	const n = 64
	msgs, base := benchTransferFixture(b, n)
	workers := 0 // GOMAXPROCS
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		st := base.Copy()
		pe := NewParallelExecutor(st, blockCtx(), workers)
		b.StartTimer()
		if _, err := pe.ApplyParallel(msgs); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelConflicting measures PE under full sender conflict (worst case re-exec).
func BenchmarkParallelConflicting(b *testing.B) {
	const n = 32
	mdb := db.OpenTest(b)
	base := state.New(mdb)
	// one rich sender, n receivers
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	base.SetBalance(sender, uint256.NewInt(1_000_000_000))
	msgs := make([]Message, n)
	for i := 0; i < n; i++ {
		rk, _ := crypto.GenerateKey()
		to := crypto.PubkeyToAddress(&rk.PublicKey)
		base.SetBalance(to, uint256.NewInt(0))
		msgs[i] = transferMsg(sender, to, 1)
	}
	if _, err := base.Commit(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		st := base.Copy()
		// re-fund sender each iter (copy already has committed base)
		pe := NewParallelExecutor(st, blockCtx(), 0)
		b.StartTimer()
		if _, err := pe.ApplyParallel(msgs); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNativeTransferPrecompile measures CALL to 0x100.
func BenchmarkNativeTransferPrecompile(b *testing.B) {
	mdb := db.OpenTest(b)
	st := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recipient := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	st.SetBalance(caller, uint256.MustFromBig(new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)))
	if _, err := st.Commit(); err != nil {
		b.Fatal(err)
	}
	var pre crypto.Address
	copy(pre[:], NativeTransferPrecompile[:])
	msg := Message{
		From:     caller,
		To:       &pre,
		Value:    uint256.NewInt(1),
		GasLimit: 100_000,
		GasPrice: big.NewInt(0),
		Data:     recipient[:],
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		fork := st.Copy()
		// top up so value transfers don't drain across iters after commit-less finalise
		fork.SetBalance(caller, uint256.MustFromBig(new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)))
		exec := NewExecutor(fork, blockCtx())
		exec.EnableDewPrecompiles(true)
		b.StartTimer()
		if _, err := exec.ApplyMessage(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func benchTransferFixture(b *testing.B, n int) ([]Message, *state.StateDB) {
	b.Helper()
	mdb := db.OpenTest(b)
	b.Cleanup(func() { mdb.Close() })
	base := state.New(mdb)
	addrs := fundEOAs(base, n*2, uint256.NewInt(1_000_000))
	if _, err := base.Commit(); err != nil {
		b.Fatal(err)
	}
	msgs := make([]Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = transferMsg(addrs[i*2], addrs[i*2+1], 1)
	}
	return msgs, base
}
