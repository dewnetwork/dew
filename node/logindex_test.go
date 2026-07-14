package node

import (
	"encoding/hex"
	"math/big"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

// Anvil #0 — same as persist_test / devnet faucet (no devnet import: cycle with node).
const (
	logIndexPrivHex0 = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	logIndexAddr0    = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	logIndexAddr1    = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
)

func logIndexGenesis() *config.Genesis {
	return &config.Genesis{
		Config: &config.ChainConfig{
			ChainID: big.NewInt(2205),
		},
		Timestamp:     0,
		GasLimit:      "0x7270e00",
		BaseFeePerGas: "0x3b9aca00",
		ExtraData:     "0x446577",
		Alloc: map[string]config.GenesisAccount{
			logIndexAddr0: {
				Balance: "1000000000000000000000000",
			},
		},
	}
}

func deployTokenTx(t *testing.T, n *Node, nonce uint64) []byte {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := hex.DecodeString(vm.TokenCreationBytecode)
	if err != nil {
		t.Fatal(err)
	}
	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		t.Fatal(err)
	}
	data := append(append([]byte{}, bin...), ctor...)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      3_000_000,
		Data:     data,
	})
	priv, err := ethcrypto.HexToECDSA(logIndexPrivHex0)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(n.ChainID()), priv)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func transferTopic() dewtypes.Hash {
	return dewtypes.BytesToHash(dewcrypto.Keccak256([]byte("Transfer(address,address,uint256)")))
}

func TestLogIndex_WriteAndFilterAfterRestart(t *testing.T) {
	g := logIndexGenesis()
	dir := filepath.Join(t.TempDir(), "chaindata")
	n1, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !n1.hasLogIndexVersion() {
		t.Fatal("fresh genesis should mark log index version")
	}

	raw := deployTokenTx(t, n1, 0)
	if _, err := n1.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}
	if n1.BlockNumber() < 1 {
		t.Fatalf("height=%d", n1.BlockNumber())
	}
	if n1.countLogIndexKeysInRange(1, n1.BlockNumber()) == 0 {
		t.Fatal("expected log-index keys after ERC-20 deploy")
	}

	logs := n1.FilterLogs(0, n1.BlockNumber(), nil, nil)
	if len(logs) == 0 {
		t.Fatal("FilterLogs empty before restart")
	}
	topic := transferTopic()
	found := false
	for _, il := range logs {
		if len(il.Log.Topics) > 0 && il.Log.Topics[0] == topic {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Transfer log not found before restart")
	}
	if err := n1.Close(); err != nil {
		t.Fatal(err)
	}

	n2, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n2.Close() })
	if !n2.hasLogIndexVersion() {
		t.Fatal("log index version missing after reopen")
	}
	if n2.countLogIndexKeysInRange(0, n2.BlockNumber()) == 0 {
		t.Fatal("log-index keys missing after reopen")
	}
	// Lazy open: allLogs empty; durable index must serve FilterLogs.
	if c := len(n2.allLogs); c != 0 {
		t.Fatalf("allLogs after lazy open=%d want 0", c)
	}
	logs2 := n2.FilterLogs(0, n2.BlockNumber(), nil, nil)
	if len(logs2) == 0 {
		t.Fatal("FilterLogs empty after restart (index path)")
	}
	found = false
	for _, il := range logs2 {
		if len(il.Log.Topics) > 0 && il.Log.Topics[0] == topic {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Transfer log not found after restart")
	}
}

func TestLogIndex_BackfillFromReceipts(t *testing.T) {
	g := logIndexGenesis()
	dir := filepath.Join(t.TempDir(), "chaindata")
	n1, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}
	raw := deployTokenTx(t, n1, 0)
	if _, err := n1.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}
	tip := n1.BlockNumber()
	if err := n1.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate pre-index chaindata: drop L keys + version meta, keep receipts.
	pdb, err := db.OpenPebble(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := pdb.Delete(metaLogIndexVersionKey); err != nil {
		t.Fatal(err)
	}
	it, ok := any(pdb).(db.IteratePrefix)
	if !ok {
		t.Fatal("pebble missing IteratePrefix")
	}
	var toDel [][]byte
	_ = it.IteratePrefix([]byte{prefixLogIndex}, func(key, value []byte) error {
		toDel = append(toDel, append([]byte(nil), key...))
		return nil
	})
	for _, k := range toDel {
		if err := pdb.Delete(k); err != nil {
			t.Fatal(err)
		}
	}
	if err := pdb.Close(); err != nil {
		t.Fatal(err)
	}

	n2, err := Open(g, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n2.Close() })
	if !n2.hasLogIndexVersion() {
		t.Fatal("ensureLogIndex did not restore version")
	}
	if n2.countLogIndexKeysInRange(0, tip) == 0 {
		t.Fatal("backfill did not rewrite log-index keys")
	}
	if len(n2.FilterLogs(0, tip, nil, nil)) == 0 {
		t.Fatal("FilterLogs empty after backfill")
	}
}

func TestLogIndex_RangeDoesNotRequireFullReceiptScan(t *testing.T) {
	g := logIndexGenesis()
	n := OpenTest(t, g)

	from := dewcrypto.MustHexToAddress(logIndexAddr0)
	to := dewcrypto.MustHexToAddress(logIndexAddr1)
	keyBytes, err := hex.DecodeString(logIndexPrivHex0)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := dewcrypto.ToECDSA(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	const nDew = 20
	for i := 0; i < nDew; i++ {
		tx := dewtypes.NewDewTx(n.ChainID(), uint64(i), from, to, uint256.NewInt(1), params.DefaultDewTxFeeWei, nil, nil)
		if err := dewtypes.SignDewTx(tx, priv); err != nil {
			t.Fatal(err)
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := n.SendDewRawTransaction(raw); err != nil {
			t.Fatalf("dew %d: %v", i, err)
		}
	}
	nonce := n.GetNonce(from)
	raw := deployTokenTx(t, n, nonce)
	if _, err := n.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}

	receipts := n.countReceiptKeys()
	if receipts < nDew+1 {
		t.Fatalf("receipts=%d want >= %d", receipts, nDew+1)
	}
	tip := n.BlockNumber()
	idxInTip := n.countLogIndexKeysInRange(tip, tip)
	if idxInTip == 0 {
		t.Fatal("expected log index keys on tip (deploy)")
	}
	idxEarly := n.countLogIndexKeysInRange(1, uint64(nDew))
	if idxEarly != 0 {
		t.Fatalf("early DewTx blocks should have 0 log-index keys, got %d", idxEarly)
	}
	allIdx := n.countLogIndexKeysInRange(0, tip)
	if allIdx >= receipts {
		t.Fatalf("log-index keys (%d) should be fewer than receipts (%d)", allIdx, receipts)
	}

	logs := n.FilterLogs(tip, tip, nil, nil)
	if len(logs) == 0 {
		t.Fatal("FilterLogs tip range empty")
	}
	tokenAddr := logs[0].Log.Address
	if len(n.FilterLogs(tip, tip, []dewcrypto.Address{tokenAddr}, nil)) == 0 {
		t.Fatal("address filter returned empty")
	}
	wrong := dewcrypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	if len(n.FilterLogs(tip, tip, []dewcrypto.Address{wrong}, nil)) != 0 {
		t.Fatal("wrong address should match nothing")
	}
}
