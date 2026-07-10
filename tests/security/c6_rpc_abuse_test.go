// Phase C6 public testnet: RPC abuse + mempool admission under spam.
// Run: go test ./tests/security/ -run C6 -count=1
package security

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/mempool"
	"github.com/dewnetwork/dew/params"
	"github.com/dewnetwork/dew/rpc"
)

func TestC6_RPC_OversizedBatchRejected(t *testing.T) {
	srv := newAbuseRPC(t)
	batch := make([]map[string]interface{}, rpc.MaxBatchItems+1)
	for i := range batch {
		batch[i] = map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_chainId",
			"params":  []interface{}{},
			"id":      i + 1,
		}
	}
	body, _ := json.Marshal(batch)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("want batch limit error, got %s", rec.Body.String())
	}
	if !strings.Contains(resp.Error.Message, "batch too large") {
		t.Fatalf("message %q", resp.Error.Message)
	}
}

func TestC6_RPC_BatchAtLimitOK(t *testing.T) {
	srv := newAbuseRPC(t)
	batch := make([]map[string]interface{}, rpc.MaxBatchItems)
	for i := range batch {
		batch[i] = map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_chainId",
			"params":  []interface{}{},
			"id":      i + 1,
		}
	}
	body, _ := json.Marshal(batch)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	var resps []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resps); err != nil {
		t.Fatalf("decode batch: %v %s", err, rec.Body.String())
	}
	if len(resps) != rpc.MaxBatchItems {
		t.Fatalf("got %d responses", len(resps))
	}
	for i, r := range resps {
		if r["error"] != nil {
			t.Fatalf("item %d error %v", i, r["error"])
		}
		if r["result"] != "0x7ea" {
			t.Fatalf("item %d result %v", i, r["result"])
		}
	}
}

func TestC6_RPC_OversizedBodyRejected(t *testing.T) {
	srv := newAbuseRPC(t)
	payload := bytes.Repeat([]byte("a"), rpc.MaxRequestBodyBytes+64)
	body := append([]byte(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1,"pad":"`), payload...)
	body = append(body, []byte(`"}`)...)
	if len(body) <= rpc.MaxRequestBodyBytes {
		t.Fatalf("test body not oversized: %d", len(body))
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	var resp struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		snippet := rec.Body.String()
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("decode: %v body=%s", err, snippet)
	}
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "too large") {
		snippet := rec.Body.String()
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		t.Fatalf("want body too large, got %s", snippet)
	}
}

func TestC6_RPC_InvalidHex_SendRaw(t *testing.T) {
	srv := newAbuseRPC(t)
	for _, raw := range []string{"not-hex", "0xZZ", "0xgg", "xyz"} {
		res, errObj := rpcCallErr(t, srv, "eth_sendRawTransaction", []interface{}{raw})
		if errObj == nil {
			t.Fatalf("raw %q expected error, got result %v", raw, res)
		}
	}
}

func TestC6_RPC_GarbagePayload_SendRaw(t *testing.T) {
	srv := newAbuseRPC(t)
	res, errObj := rpcCallErr(t, srv, "eth_sendRawTransaction", []interface{}{"0xdeadbeef"})
	if errObj == nil {
		t.Fatalf("expected invalid tx error, got %v", res)
	}
	res, errObj = rpcCallErr(t, srv, "dew_sendRawTransaction", []interface{}{"0x02c0"})
	if errObj == nil {
		t.Fatalf("EVM typed prefix must fail dew_sendRawTransaction, got %v", res)
	}
}

func TestC6_RPC_SpamUnderpricedRejected(t *testing.T) {
	n := newTestNode(t)
	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	chainID := big.NewInt(int64(params.PublicTestnetChainID))
	for i := 0; i < 32; i++ {
		tx := ethtypes.NewTransaction(uint64(i), to, big.NewInt(1), 21_000, big.NewInt(1), nil)
		signed, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), key)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := signed.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := n.SendRawTransaction(raw); err == nil {
			t.Fatal("expected underpriced rejection")
		}
	}
}

func TestC6_RPC_SpamOversizedTxRejected(t *testing.T) {
	n := newTestNode(t)
	cfg := mempool.DefaultConfig()
	cfg.MaxTxBytes = 256
	n.SetMempoolConfig(cfg)

	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	chainID := big.NewInt(int64(params.PublicTestnetChainID))
	data := make([]byte, 512)
	tx := ethtypes.NewTransaction(0, to, big.NewInt(0), 100_000, new(big.Int).SetUint64(params.PublicTestnetMinGasPriceWei), data)
	signed, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= cfg.MaxTxBytes {
		t.Fatalf("raw %d not larger than max %d", len(raw), cfg.MaxTxBytes)
	}
	_, err = n.SendRawTransaction(raw)
	if err == nil {
		t.Fatal("expected oversized rejection")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestC6_Mempool_PerSenderLimitWithoutMine(t *testing.T) {
	cfg := mempool.DefaultConfig()
	cfg.MaxPerSender = 3
	cfg.MaxGlobal = 100
	pool := mempool.New(cfg)
	chainID := big.NewInt(int64(params.PublicTestnetChainID))
	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	fromDew := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	for i := 0; i < 3; i++ {
		tx := ethtypes.NewTransaction(uint64(i), to, big.NewInt(1), 21_000, new(big.Int).SetUint64(params.PublicTestnetMinGasPriceWei), nil)
		signed, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), key)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := signed.MarshalBinary()
		if _, err := pool.AddEVM(signed, fromDew, raw, chainID); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
	}
	tx := ethtypes.NewTransaction(3, to, big.NewInt(1), 21_000, new(big.Int).SetUint64(params.PublicTestnetMinGasPriceWei), nil)
	signed, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := signed.MarshalBinary()
	_, err = pool.AddEVM(signed, fromDew, raw, chainID)
	if err == nil {
		t.Fatal("expected per-sender limit")
	}
	if !strings.Contains(err.Error(), "per-sender") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestC6_DewTx_SpamWrongFee(t *testing.T) {
	n := newTestNode(t)
	key, err := crypto.ToECDSA(mustDecodeHex("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"))
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	for i := 0; i < 16; i++ {
		tx := types.NewDewTx(big.NewInt(int64(params.PublicTestnetChainID)), uint64(i), sender, recv, uint256.NewInt(1), 1, nil, nil)
		tx.Fee = 1 // below MinDewTxFeeWei
		if err := types.SignDewTx(tx, key); err != nil {
			t.Fatal(err)
		}
		raw, _ := tx.MarshalBinary()
		if _, err := n.SendDewRawTransaction(raw); err == nil {
			t.Fatal("expected underpriced DewTx rejection")
		}
	}
}

func TestC6_FreezeSurfaceDocumentedInParams(t *testing.T) {
	if rpc.MaxBatchItems != params.PublicTestnetMaxRPCBatch {
		t.Fatal("batch limit drift")
	}
	if rpc.MaxRequestBodyBytes != params.PublicTestnetMaxRPCBodyBytes {
		t.Fatal("body limit drift")
	}
	if params.PublicTestnetFreezeTag != "public-testnet-v1" {
		t.Fatal(params.PublicTestnetFreezeTag)
	}
}

func newAbuseRPC(t *testing.T) *rpc.Server {
	t.Helper()
	n := newTestNode(t)
	srv := rpc.NewServer()
	srv.RegisterAll(rpc.NewAPI(n).Handlers())
	return srv
}

func rpcCallErr(t *testing.T, srv *rpc.Server, method string, params interface{}) (interface{}, map[string]interface{}) {
	t.Helper()
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	if params == nil {
		payload["params"] = []interface{}{}
	}
	body, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	var resp struct {
		Result interface{}            `json:"result"`
		Error  map[string]interface{} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return resp.Result, resp.Error
}
