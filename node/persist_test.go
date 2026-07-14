package node

import (
	"encoding/hex"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

func testGenesis() *config.Genesis {
	return &config.Genesis{
		Config: &config.ChainConfig{
			ChainID: big.NewInt(2205),
		},
		Timestamp:     0,
		GasLimit:      "0x7270e00",
		BaseFeePerGas: "0x3b9aca00",
		ExtraData:     "0x446577",
		Alloc: map[string]config.GenesisAccount{
			"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {
				Balance: "1000000000000000000000000",
			},
		},
	}
}

func TestNode_RestartRecoversTip(t *testing.T) {
	g := testGenesis()
	dir := filepath.Join(t.TempDir(), "chaindata")

	n1, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}

	from := dewcrypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := dewcrypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	balBefore := new(uint256.Int).Set(n1.GetBalance(from))

	// Build a simple signed-like DewTx path via native auto-mine is heavy; import empty-ish
	// blocks by sealing via statedb credit is complex. Use ImportCommittedBlock with empty txs
	// after manually adjusting — simpler: transfer via SendRawTransaction needs signature.
	// Use native DewTx through SendDewRawTransaction with a properly signed tx from crypto.
	keyBytes, err := hex.DecodeString("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	priv, err := dewcrypto.ToECDSA(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	tx := dewtypes.NewDewTx(big.NewInt(2205), 0, from, to, uint256.NewInt(1_000_000_000_000_000_000), params.DefaultDewTxFeeWei, nil, nil)
	if err := dewtypes.SignDewTx(tx, priv); err != nil {
		t.Fatal(err)
	}
	txRaw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	txHash, err := n1.SendDewRawTransaction(txRaw)
	if err != nil {
		t.Fatal(err)
	}
	if n1.BlockNumber() != 1 {
		t.Fatalf("height=%d want 1", n1.BlockNumber())
	}
	tipHash := n1.CurrentHeader().Hash()
	balAfter := n1.GetBalance(from)
	if balAfter.Cmp(balBefore) >= 0 {
		t.Fatalf("sender balance did not decrease")
	}
	if n1.GetReceipt(txHash) == nil {
		t.Fatal("missing receipt")
	}
	if err := n1.Close(); err != nil {
		t.Fatal(err)
	}

	n2, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}

	if n2.BlockNumber() != 1 {
		t.Fatalf("reopen height=%d want 1", n2.BlockNumber())
	}
	if n2.CurrentHeader().Hash() != tipHash {
		t.Fatalf("tip hash mismatch after reopen")
	}
	if n2.GetBalance(from).Cmp(balAfter) != 0 {
		t.Fatalf("balance mismatch after reopen")
	}
	if n2.GetReceipt(txHash) == nil {
		t.Fatal("receipt missing after reopen")
	}
	if n2.GetTransaction(txHash) == nil {
		t.Fatal("tx index missing after reopen")
	}
	blk := n2.GetBlockByNumber(1)
	if blk == nil {
		t.Fatal("block 1 missing")
	}
}

// TestNode_LazyHydrateLargeTip ensures Open loads only the tip into RAM and still
// serves historical blocks / receipts / tx index on demand (Track 4).
func TestNode_LazyHydrateLargeTip(t *testing.T) {
	g := testGenesis()
	dir := filepath.Join(t.TempDir(), "chaindata")

	n1, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := hex.DecodeString("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	priv, err := dewcrypto.ToECDSA(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	from := dewcrypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := dewcrypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	const nBlocks = 24
	var firstHash, midHash, lastHash dewtypes.Hash
	for i := 0; i < nBlocks; i++ {
		tx := dewtypes.NewDewTx(big.NewInt(2205), uint64(i), from, to, uint256.NewInt(1), params.DefaultDewTxFeeWei, nil, nil)
		if err := dewtypes.SignDewTx(tx, priv); err != nil {
			t.Fatal(err)
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		h, err := n1.SendDewRawTransaction(raw)
		if err != nil {
			t.Fatalf("tx %d: %v", i, err)
		}
		switch i {
		case 0:
			firstHash = h
		case nBlocks / 2:
			midHash = h
		case nBlocks - 1:
			lastHash = h
		}
	}
	if n1.BlockNumber() != uint64(nBlocks) {
		t.Fatalf("height=%d want %d", n1.BlockNumber(), nBlocks)
	}
	tipHash := n1.CurrentHeader().Hash()
	if err := n1.Close(); err != nil {
		t.Fatal(err)
	}

	n2, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n2.Close() })

	if n2.BlockNumber() != uint64(nBlocks) {
		t.Fatalf("reopen height=%d want %d", n2.BlockNumber(), nBlocks)
	}
	if n2.CurrentHeader().Hash() != tipHash {
		t.Fatal("tip hash mismatch after lazy open")
	}
	// Lazy: only tip block cached at open (not full 0..tip).
	if c := n2.cachedBlockCount(); c != 1 {
		t.Fatalf("cached blocks after open=%d want 1 (tip only)", c)
	}

	// Historical heights load on demand.
	if n2.GetBlockByNumber(1) == nil {
		t.Fatal("block 1 missing after lazy load")
	}
	if n2.GetBlockByNumber(uint64(nBlocks/2)) == nil {
		t.Fatal("mid block missing")
	}
	if n2.GetBlockByNumber(0) == nil {
		t.Fatal("genesis block missing")
	}
	if c := n2.cachedBlockCount(); c < 3 {
		t.Fatalf("expected cache growth after historical loads, got %d", c)
	}

	if n2.GetReceipt(firstHash) == nil || n2.GetTransaction(firstHash) == nil {
		t.Fatal("first tx receipt/index missing on demand")
	}
	if n2.GetReceipt(midHash) == nil || n2.GetTransaction(midHash) == nil {
		t.Fatal("mid tx receipt/index missing on demand")
	}
	if n2.GetReceipt(lastHash) == nil || n2.GetTransaction(lastHash) == nil {
		t.Fatal("last tx receipt/index missing on demand")
	}

	looks := n2.TransactionsInBlock(1)
	if len(looks) == 0 {
		t.Fatal("TransactionsInBlock(1) empty after lazy open")
	}
	// DewTx receipts may carry empty Logs; FilterLogs must still succeed without panic.
	_ = n2.FilterLogs(0, uint64(nBlocks), nil, nil)
}

func TestNode_GenesisMismatchRefusesOpen(t *testing.T) {
	g1 := testGenesis()
	dir := filepath.Join(t.TempDir(), "chaindata")
	n, err := Open(g1, dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = n.Close()

	g2 := testGenesis()
	g2.Alloc["0x70997970C51812dc3A010C7d01b50e0d17dc79C8"] = config.GenesisAccount{
		Balance: "1",
	}
	_, err = Open(g2, dir)
	if err == nil {
		t.Fatal("expected genesis mismatch error")
	}
}

func TestNode_OpenTestFreshGenesis(t *testing.T) {
	n := OpenTest(t, testGenesis())
	if n.BlockNumber() != 0 {
		t.Fatalf("height=%d", n.BlockNumber())
	}
}
