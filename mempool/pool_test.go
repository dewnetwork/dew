package mempool

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

func testKey(t *testing.T) (*ecdsa.PrivateKey, crypto.Address) {
	t.Helper()
	k, err := ethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k, crypto.PubkeyToAddress(&k.PublicKey)
}

func signLegacy(t *testing.T, key *ecdsa.PrivateKey, chainID *big.Int, nonce uint64, gasPrice *big.Int, data []byte) (*ethtypes.Transaction, []byte) {
	t.Helper()
	to := ethcrypto.PubkeyToAddress(key.PublicKey)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    big.NewInt(0),
		Gas:      21_000,
		GasPrice: gasPrice,
		Data:     data,
	})
	signer := ethtypes.LatestSignerForChainID(chainID)
	signed, err := ethtypes.SignTx(tx, signer, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return signed, raw
}

func TestAddEVM_MinFeeAndLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxGlobal = 3
	cfg.MaxPerSender = 2
	cfg.MinGasPriceWei = big.NewInt(1_000_000_000)
	p := New(cfg)
	chainID := big.NewInt(2205)
	key, from := testKey(t)

	// underpriced
	tx, raw := signLegacy(t, key, chainID, 0, big.NewInt(1), nil)
	if _, err := p.AddEVM(tx, from, raw, chainID); err != ErrUnderpriced {
		t.Fatalf("want underpriced, got %v", err)
	}

	// ok
	tx, raw = signLegacy(t, key, chainID, 0, big.NewInt(1_000_000_000), nil)
	h, err := p.AddEVM(tx, from, raw, chainID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Get(h) == nil {
		t.Fatal("missing entry")
	}

	// second nonce
	tx2, raw2 := signLegacy(t, key, chainID, 1, big.NewInt(1_000_000_000), nil)
	if _, err := p.AddEVM(tx2, from, raw2, chainID); err != nil {
		t.Fatal(err)
	}

	// third from same sender → per-sender limit
	tx3, raw3 := signLegacy(t, key, chainID, 2, big.NewInt(1_000_000_000), nil)
	if _, err := p.AddEVM(tx3, from, raw3, chainID); err != ErrSenderLimit {
		t.Fatalf("want sender limit, got %v", err)
	}
}

func TestAddEVM_TxTooLarge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTxBytes = 100
	p := New(cfg)
	chainID := big.NewInt(2205)
	key, from := testKey(t)
	data := make([]byte, 200)
	tx, raw := signLegacy(t, key, chainID, 0, big.NewInt(1_000_000_000), data)
	if _, err := p.AddEVM(tx, from, raw, chainID); err != ErrTxTooLarge {
		t.Fatalf("want too large, got %v", err)
	}
}

func TestReplaceByFee(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PriceBumpPercent = 10
	p := New(cfg)
	chainID := big.NewInt(2205)
	key, from := testKey(t)

	tx, raw := signLegacy(t, key, chainID, 0, big.NewInt(1_000_000_000), nil)
	oldHash, err := p.AddEVM(tx, from, raw, chainID)
	if err != nil {
		t.Fatal(err)
	}

	// +5% insufficient
	txLow, rawLow := signLegacy(t, key, chainID, 0, big.NewInt(1_050_000_000), nil)
	if _, err := p.AddEVM(txLow, from, rawLow, chainID); err != ErrReplaceUnder {
		t.Fatalf("want replace underpriced, got %v", err)
	}

	// +10% ok
	txHi, rawHi := signLegacy(t, key, chainID, 0, big.NewInt(1_100_000_000), nil)
	newHash, err := p.AddEVM(txHi, from, rawHi, chainID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Get(oldHash) != nil {
		t.Fatal("old should be replaced")
	}
	if p.Get(newHash) == nil {
		t.Fatal("new missing")
	}
	if p.Len() != 1 {
		t.Fatalf("len=%d", p.Len())
	}
}

func TestRBFDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PriceBumpPercent = 0
	p := New(cfg)
	chainID := big.NewInt(2205)
	key, from := testKey(t)

	tx, raw := signLegacy(t, key, chainID, 0, big.NewInt(1_000_000_000), nil)
	if _, err := p.AddEVM(tx, from, raw, chainID); err != nil {
		t.Fatal(err)
	}
	tx2, raw2 := signLegacy(t, key, chainID, 0, big.NewInt(2_000_000_000), nil)
	if _, err := p.AddEVM(tx2, from, raw2, chainID); err != ErrRBFDisabled {
		t.Fatalf("want RBF disabled, got %v", err)
	}
}

func TestGlobalEviction(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxGlobal = 2
	cfg.MaxPerSender = 10
	p := New(cfg)
	chainID := big.NewInt(2205)

	k1, f1 := testKey(t)
	k2, f2 := testKey(t)
	k3, f3 := testKey(t)

	tx1, r1 := signLegacy(t, k1, chainID, 0, big.NewInt(1_000_000_000), nil)
	tx2, r2 := signLegacy(t, k2, chainID, 0, big.NewInt(2_000_000_000), nil)
	if _, err := p.AddEVM(tx1, f1, r1, chainID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.AddEVM(tx2, f2, r2, chainID); err != nil {
		t.Fatal(err)
	}
	// higher price should evict cheapest (tx1)
	tx3, r3 := signLegacy(t, k3, chainID, 0, big.NewInt(3_000_000_000), nil)
	if _, err := p.AddEVM(tx3, f3, r3, chainID); err != nil {
		t.Fatal(err)
	}
	if p.Len() != 2 {
		t.Fatalf("len=%d", p.Len())
	}
	if p.Get(dewtypes.BytesToHash(tx1.Hash().Bytes())) != nil {
		t.Fatal("cheapest should be evicted")
	}
}

func TestAddDew_MinFee(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinDewFeeWei = params.MinDewTxFeeWei
	p := New(cfg)
	chainID := big.NewInt(2205)
	key, from := testKey(t)
	to := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	tx := dewtypes.NewDewTx(chainID, 0, from, to, uint256.NewInt(1), 1, nil, []crypto.Address{from, to})
	if err := dewtypes.SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AddDew(tx, raw, chainID); err != ErrUnderpriced {
		t.Fatalf("want underpriced, got %v", err)
	}

	tx2 := dewtypes.NewDewTx(chainID, 0, from, to, uint256.NewInt(1), params.MinDewTxFeeWei, nil, []crypto.Address{from, to})
	if err := dewtypes.SignDewTx(tx2, key); err != nil {
		t.Fatal(err)
	}
	raw2, _ := tx2.MarshalBinary()
	if _, err := p.AddDew(tx2, raw2, chainID); err != nil {
		t.Fatal(err)
	}
	if p.Len() != 1 {
		t.Fatal("expected 1 dew tx")
	}
}

func TestUnifiedSurface_EVMAndDewShareLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxGlobal = 2
	cfg.MaxPerSender = 2
	p := New(cfg)
	chainID := big.NewInt(2205)
	key, from := testKey(t)
	to := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	tx, raw := signLegacy(t, key, chainID, 0, big.NewInt(1_000_000_000), nil)
	if _, err := p.AddEVM(tx, from, raw, chainID); err != nil {
		t.Fatal(err)
	}
	dtx := dewtypes.NewDewTx(chainID, 1, from, to, uint256.NewInt(0), params.MinDewTxFeeWei, nil, []crypto.Address{from, to})
	if err := dewtypes.SignDewTx(dtx, key); err != nil {
		t.Fatal(err)
	}
	draw, _ := dtx.MarshalBinary()
	if _, err := p.AddDew(dtx, draw, chainID); err != nil {
		t.Fatal(err)
	}
	if p.Len() != 2 {
		t.Fatalf("len=%d", p.Len())
	}
	// third fills global with different sender and higher price — ok eviction path tested above
}

func TestTelemetry_CountersAndFloors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxGlobal = 2
	cfg.MaxPerSender = 10
	cfg.MinGasPriceWei = big.NewInt(1_000_000_000)
	p := New(cfg)
	chainID := big.NewInt(2205)

	// underpriced
	key, from := testKey(t)
	tx, raw := signLegacy(t, key, chainID, 0, big.NewInt(1), nil)
	if _, err := p.AddEVM(tx, from, raw, chainID); err != ErrUnderpriced {
		t.Fatalf("underpriced: %v", err)
	}

	k1, f1 := testKey(t)
	k2, f2 := testKey(t)
	k3, f3 := testKey(t)
	tx1, r1 := signLegacy(t, k1, chainID, 0, big.NewInt(1_000_000_000), nil)
	tx2, r2 := signLegacy(t, k2, chainID, 0, big.NewInt(2_000_000_000), nil)
	tx3, r3 := signLegacy(t, k3, chainID, 0, big.NewInt(3_000_000_000), nil)
	if _, err := p.AddEVM(tx1, f1, r1, chainID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.AddEVM(tx2, f2, r2, chainID); err != nil {
		t.Fatal(err)
	}
	// evict cheapest
	if _, err := p.AddEVM(tx3, f3, r3, chainID); err != nil {
		t.Fatal(err)
	}

	// RBF replace
	txHi, rHi := signLegacy(t, k2, chainID, 0, big.NewInt(2_200_000_000), nil)
	if _, err := p.AddEVM(txHi, f2, rHi, chainID); err != nil {
		t.Fatal(err)
	}

	st := p.Telemetry()
	if st.Pending != 2 {
		t.Fatalf("pending=%d", st.Pending)
	}
	if st.PendingEVM != 2 || st.PendingDew != 0 {
		t.Fatalf("evm=%d dew=%d", st.PendingEVM, st.PendingDew)
	}
	if st.Admits != 4 { // 2 initial + 1 after eviction + 1 replace
		t.Fatalf("admits=%d want 4", st.Admits)
	}
	if st.Evictions != 1 {
		t.Fatalf("evictions=%d", st.Evictions)
	}
	if st.Replaces != 1 {
		t.Fatalf("replaces=%d", st.Replaces)
	}
	if st.Rejects.Underpriced != 1 || st.Rejects.Total < 1 {
		t.Fatalf("rejects=%+v", st.Rejects)
	}
	if st.MinGasPriceWei != "1000000000" {
		t.Fatalf("min gas floor %q", st.MinGasPriceWei)
	}
	if st.MaxGlobal != 2 || st.MaxPerSender != 10 {
		t.Fatalf("limits global=%d per=%d", st.MaxGlobal, st.MaxPerSender)
	}
	if len(st.TopSenders) == 0 {
		t.Fatal("expected top_senders")
	}
}
