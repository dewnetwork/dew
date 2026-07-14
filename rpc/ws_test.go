package rpc

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

func testWSGenesis() *config.Genesis {
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

func TestRPC_EthSubscribe_HTTPRejected(t *testing.T) {
	n := node.OpenTest(t, testWSGenesis())
	srv := NewServer()
	api := NewAPI(n)
	srv.RegisterAll(api.Handlers())
	srv.EnableSubscriptions(n, api)

	body := `{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
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
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "WebSocket") {
		t.Fatalf("want WebSocket-only error, got %+v body=%s", resp.Error, rr.Body.String())
	}
}

func TestRPC_WS_NewHeads(t *testing.T) {
	n := node.OpenTest(t, testWSGenesis())
	srv := NewServer()
	api := NewAPI(n)
	srv.RegisterAll(api.Handlers())
	srv.EnableSubscriptions(n, api)

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	subReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_subscribe",
		"params":  []interface{}{"newHeads"},
	}
	if err := conn.WriteJSON(subReq); err != nil {
		t.Fatal(err)
	}
	var subResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := conn.ReadJSON(&subResp); err != nil {
		t.Fatal(err)
	}
	if subResp.Error != nil {
		t.Fatal(subResp.Error.Message)
	}
	if subResp.Result == "" {
		t.Fatal("empty subscription id")
	}

	from := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	keyBytes, err := hex.DecodeString("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	priv, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	tx := types.NewDewTx(big.NewInt(2205), 0, from, to, uint256.NewInt(1), params.DefaultDewTxFeeWei, nil, nil)
	if err := types.SignDewTx(tx, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.SendDewRawTransaction(raw); err != nil {
		t.Fatal(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var note struct {
		Method string `json:"method"`
		Params struct {
			Subscription string                 `json:"subscription"`
			Result       map[string]interface{} `json:"result"`
		} `json:"params"`
	}
	if err := conn.ReadJSON(&note); err != nil {
		t.Fatal(err)
	}
	if note.Method != "eth_subscription" {
		t.Fatalf("method=%s", note.Method)
	}
	if note.Params.Subscription != subResp.Result {
		t.Fatalf("sub id mismatch")
	}
	if note.Params.Result["number"] == nil {
		t.Fatalf("missing number in head: %+v", note.Params.Result)
	}

	unsub := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "eth_unsubscribe",
		"params":  []interface{}{subResp.Result},
	}
	if err := conn.WriteJSON(unsub); err != nil {
		t.Fatal(err)
	}
	var unsubResp struct {
		Result bool `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := conn.ReadJSON(&unsubResp); err != nil {
		t.Fatal(err)
	}
	if unsubResp.Error != nil || !unsubResp.Result {
		t.Fatalf("unsubscribe failed: %+v", unsubResp)
	}
}
