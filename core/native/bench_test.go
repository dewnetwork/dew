package native

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

// BenchmarkDewTxTransfer measures native system token transfers.
func BenchmarkDewTxTransfer(b *testing.B) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	st := state.New(mdb)
	key, err := crypto.GenerateKey()
	if err != nil {
		b.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	sink := crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0")
	// enough for b.N transfers
	need := uint64(b.N)*params.DefaultDewTxFeeWei + uint64(b.N)*10 + 1_000_000
	st.SetBalance(sender, uint256.NewInt(need))
	if _, err := st.Commit(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx := types.NewDewTx(big.NewInt(2026), uint64(i), sender, recv, uint256.NewInt(10), params.DefaultDewTxFeeWei, nil, nil)
		if err := types.SignDewTx(tx, key); err != nil {
			b.Fatal(err)
		}
		exec := NewExecutor(st, sink)
		res, err := exec.ApplyDewTx(tx)
		if err != nil || res.Failed {
			b.Fatalf("i=%d err=%v res=%+v", i, err, res)
		}
	}
}
