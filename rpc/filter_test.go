package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/node"
)

func filterTestGenesis() *config.Genesis {
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

func filterServer(t *testing.T) (*node.Node, *Server) {
	t.Helper()
	n := node.OpenTest(t, filterTestGenesis())
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())
	return n, srv
}

func rpcCallErr(t *testing.T, srv *Server, method string, params interface{}) string {
	t.Helper()
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	if params == nil {
		body["params"] = []interface{}{}
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	var resp struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatalf("%s: expected error, body=%s", method, rr.Body.String())
	}
	return resp.Error.Message
}

func sendTestTransfer(t *testing.T, srv *Server, nonce uint64) {
	t.Helper()
	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	signer := ethtypes.LatestSignerForChainID(big.NewInt(2205))
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(2205),
		Nonce:     nonce,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(2_000_000_000),
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(1e15),
	})
	signed, err := ethtypes.SignTx(tx, signer, key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	res := rpcCall(t, srv, "eth_sendRawTransaction", []interface{}{EncodeBytes(raw)})
	if s, ok := res.(string); !ok || len(s) != 66 {
		t.Fatalf("sendRaw = %v", res)
	}
}

func buildTokenDeployRaw(t *testing.T, nonce uint64) []byte {
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
	priv, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(big.NewInt(2205)), priv)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRPC_Filter_NewAndUninstall(t *testing.T) {
	_, srv := filterServer(t)

	id := rpcCall(t, srv, "eth_newFilter", []interface{}{map[string]interface{}{}})
	idStr, ok := id.(string)
	if !ok || len(idStr) < 4 || idStr[:2] != "0x" {
		t.Fatalf("filter id = %v", id)
	}

	okRes := rpcCall(t, srv, "eth_uninstallFilter", []interface{}{idStr})
	if okRes != true {
		t.Fatalf("uninstall first = %v", okRes)
	}
	okRes = rpcCall(t, srv, "eth_uninstallFilter", []interface{}{idStr})
	if okRes != false {
		t.Fatalf("uninstall second = %v", okRes)
	}
}

func TestRPC_Filter_UnknownChangesErrors(t *testing.T) {
	_, srv := filterServer(t)
	msg := rpcCallErr(t, srv, "eth_getFilterChanges", []interface{}{"0xdeadbeefdeadbeef"})
	if !strings.Contains(msg, "filter not found") {
		t.Fatalf("want filter not found, got %q", msg)
	}
}

func TestRPC_Filter_BlockFilterChanges(t *testing.T) {
	_, srv := filterServer(t)

	id := rpcCall(t, srv, "eth_newBlockFilter", []interface{}{}).(string)

	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok := ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("empty changes = %v", ch)
	}

	sendTestTransfer(t, srv, 0)

	ch = rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok = ch.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("want 1 block hash, got %v", ch)
	}
	hash, ok := arr[0].(string)
	if !ok || len(hash) != 66 {
		t.Fatalf("hash = %v", arr[0])
	}

	ch = rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok = ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("second poll want empty, got %v", ch)
	}
}

func TestRPC_Filter_PendingEmpty(t *testing.T) {
	_, srv := filterServer(t)
	id := rpcCall(t, srv, "eth_newPendingTransactionFilter", []interface{}{}).(string)
	sendTestTransfer(t, srv, 0)
	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok := ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("pending changes = %v", ch)
	}
}

func TestRPC_Filter_LogFilterChanges(t *testing.T) {
	_, srv := filterServer(t)

	id := rpcCall(t, srv, "eth_newFilter", []interface{}{map[string]interface{}{}}).(string)

	raw := buildTokenDeployRaw(t, 0)
	rpcCall(t, srv, "eth_sendRawTransaction", []interface{}{EncodeBytes(raw)})

	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok := ch.([]interface{})
	if !ok || len(arr) < 1 {
		t.Fatalf("want >=1 log, got %v", ch)
	}
	log0, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("log0 type = %T", arr[0])
	}
	if log0["address"] == nil || log0["transactionHash"] == nil {
		t.Fatalf("incomplete log: %v", log0)
	}

	ch = rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok = ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("second poll = %v", ch)
	}
}

func TestRPC_Filter_GetFilterLogs_DoesNotAdvanceCursor(t *testing.T) {
	_, srv := filterServer(t)

	// Deploy token first (logs at height 1).
	raw := buildTokenDeployRaw(t, 0)
	rpcCall(t, srv, "eth_sendRawTransaction", []interface{}{EncodeBytes(raw)})

	// Filter created at tip — poll has no new blocks.
	id := rpcCall(t, srv, "eth_newFilter", []interface{}{map[string]interface{}{}}).(string)

	logs := rpcCall(t, srv, "eth_getFilterLogs", []interface{}{id})
	arr, ok := logs.([]interface{})
	if !ok || len(arr) < 1 {
		t.Fatalf("getFilterLogs = %v", logs)
	}

	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	chArr, ok := ch.([]interface{})
	if !ok || len(chArr) != 0 {
		t.Fatalf("changes after getFilterLogs should be empty, got %v", ch)
	}
}

func TestRPC_Filter_GetFilterLogs_NotLogFilter(t *testing.T) {
	_, srv := filterServer(t)
	id := rpcCall(t, srv, "eth_newBlockFilter", []interface{}{}).(string)
	msg := rpcCallErr(t, srv, "eth_getFilterLogs", []interface{}{id})
	if !strings.Contains(msg, "not a log filter") {
		t.Fatalf("want not a log filter, got %q", msg)
	}
}

func TestFilterStore_MaxFilters(t *testing.T) {
	s := newFilterStore()
	for i := 0; i < MaxFilters; i++ {
		if _, err := s.install(&installedFilter{kind: filterBlocks, lastPolled: 0}); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	if _, err := s.install(&installedFilter{kind: filterBlocks}); err == nil {
		t.Fatal("want capacity error")
	}
}
