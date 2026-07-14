package indexer_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/indexer"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
	"github.com/dewnetwork/dew/rpc"
)

func testGenesis() *config.Genesis {
	return &config.Genesis{
		Config:        &config.ChainConfig{ChainID: big.NewInt(2205)},
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

func startRPCBG(t *testing.T, n *node.Node) string {
	t.Helper()
	srv := rpc.NewServer()
	api := rpc.NewAPI(n)
	srv.RegisterAll(api.Handlers())
	srv.EnableSubscriptions(n, api)
	go func() {
		_ = srv.ListenAndServe("127.0.0.1:0")
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a := srv.Addr(); a != "" {
			t.Cleanup(func() { _ = srv.Close() })
			return "http://" + a
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("rpc did not start")
	return ""
}

func TestIndexer_CatchUpAndAddressAPI(t *testing.T) {
	n := node.OpenTest(t, testGenesis())
	rpcURL := startRPCBG(t, n)

	from := dewcrypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := dewcrypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	keyBytes, err := hex.DecodeString("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	priv, err := dewcrypto.ToECDSA(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		tx := dewtypes.NewDewTx(big.NewInt(2205), uint64(i), from, to, uint256.NewInt(1), params.DefaultDewTxFeeWei, nil, nil)
		if err := dewtypes.SignDewTx(tx, priv); err != nil {
			t.Fatal(err)
		}
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := n.SendDewRawTransaction(raw); err != nil {
			t.Fatal(err)
		}
	}

	nonce := n.GetNonce(from)
	ethKey, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	legacy := ethtypes.NewTransaction(
		nonce,
		common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8"),
		big.NewInt(1),
		21_000,
		big.NewInt(1_000_000_000),
		nil,
	)
	signed, err := ethtypes.SignTx(legacy, ethtypes.LatestSignerForChainID(big.NewInt(2205)), ethKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(t.TempDir(), "idx.db")
	ix, err := indexer.New(indexer.Config{
		RPCURL:           rpcURL,
		SQLitePath:       dbPath,
		MaxBlocksPerTick: 128,
		PollInterval:     50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ix.CatchUpOnce(ctx); err != nil {
		t.Fatal(err)
	}

	srv, err := indexer.NewServer(ix, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Start() }()
	// brief settle
	time.Sleep(20 * time.Millisecond)
	base := srv.ListenURL()

	res, err := http.Get(base + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d %s", res.StatusCode, body)
	}
	var st indexer.Status
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatal(err)
	}
	if st.Indexed < 1 {
		t.Fatalf("indexed=%d", st.Indexed)
	}

	url := fmt.Sprintf("%s/v1/address/%s/txs?limit=20", base, from.Hex())
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("txs %d %s", res.StatusCode, body)
	}
	var payload struct {
		Txs []indexer.TxRow `json:"txs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Txs) < 3 {
		t.Fatalf("want >=3 txs, got %d body=%s", len(payload.Txs), body)
	}

	res, err = http.Get(base + "/v1/stats/volume?from=0")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("volume %d %s", res.StatusCode, body)
	}
}
