package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

func TestRPC_ChainIdAndBalance(t *testing.T) {
	g, err := config.LoadGenesisFile("../genesis.json")
	if err != nil {
		// fallback parse sample
		g, err = config.ParseGenesis([]byte(`{
		  "config": {"chainId": 2205, "homesteadBlock": 0, "eip150Block": 0, "eip155Block": 0, "eip158Block": 0,
		    "byzantiumBlock": 0, "constantinopleBlock": 0, "petersburgBlock": 0, "istanbulBlock": 0,
		    "muirGlacierBlock": 0, "berlinBlock": 0, "londonBlock": 0, "shanghaiBlock": 0, "cancunBlock": 0},
		  "timestamp": 0, "extraData": "0x", "gasLimit": "0x7270e00", "baseFeePerGas": "0x3b9aca00",
		  "alloc": {
		    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {"balance": "1000000000000000000000000"}
		  }
		}`))
		if err != nil {
			t.Fatal(err)
		}
	}
	n := node.OpenTest(t, g)
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	// eth_chainId
	res := rpcCall(t, srv, "eth_chainId", nil)
	if res != "0x89d" { // 2205
		t.Fatalf("chainId = %v, want 0x89d", res)
	}

	// eth_blockNumber genesis
	res = rpcCall(t, srv, "eth_blockNumber", nil)
	if res != "0x0" {
		t.Fatalf("blockNumber = %v", res)
	}

	// eth_getBalance
	res = rpcCall(t, srv, "eth_getBalance", []interface{}{
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", "latest",
	})
	balStr, ok := res.(string)
	if !ok || balStr == "0x0" {
		t.Fatalf("balance = %v", res)
	}

	// net_version
	res = rpcCall(t, srv, "net_version", nil)
	if res != "2205" {
		t.Fatalf("net_version = %v", res)
	}
}

func TestRPC_SendRawTransaction_Transfer(t *testing.T) {
	g, err := config.ParseGenesis([]byte(`{
	  "config": {"chainId": 2205, "homesteadBlock": 0, "eip150Block": 0, "eip155Block": 0, "eip158Block": 0,
	    "byzantiumBlock": 0, "constantinopleBlock": 0, "petersburgBlock": 0, "istanbulBlock": 0,
	    "muirGlacierBlock": 0, "berlinBlock": 0, "londonBlock": 0, "shanghaiBlock": 0, "cancunBlock": 0},
	  "timestamp": 0, "extraData": "0x", "gasLimit": "0x7270e00", "baseFeePerGas": "0x3b9aca00",
	  "alloc": {
	    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {"balance": "1000000000000000000000000"}
	  }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	n := node.OpenTest(t, g)
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	// Anvil key #0
	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	from := ethcrypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	signer := ethtypes.LatestSignerForChainID(big.NewInt(2205))
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(2205),
		Nonce:     0,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(2_000_000_000), // > base fee 1 gwei
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(1e15), // 0.001 eth
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
	txHash, ok := res.(string)
	if !ok || len(txHash) != 66 {
		t.Fatalf("sendRaw = %v", res)
	}

	// head advanced
	bn := rpcCall(t, srv, "eth_blockNumber", nil)
	if bn != "0x1" {
		t.Fatalf("blockNumber after tx = %v", bn)
	}

	// receipt
	rcpt := rpcCall(t, srv, "eth_getTransactionReceipt", []interface{}{txHash})
	m, ok := rcpt.(map[string]interface{})
	if !ok {
		t.Fatalf("receipt = %v", rcpt)
	}
	if m["status"] != "0x1" {
		t.Fatalf("status = %v", m["status"])
	}

	// recipient balance
	bal := rpcCall(t, srv, "eth_getBalance", []interface{}{to.Hex(), "latest"})
	if bal == "0x0" {
		t.Fatal("recipient still zero")
	}

	// nonce advanced
	nonce := rpcCall(t, srv, "eth_getTransactionCount", []interface{}{from.Hex(), "latest"})
	if nonce != "0x1" {
		t.Fatalf("nonce = %v", nonce)
	}

	_ = time.Now()
}

func TestRPC_DewSendRawTransaction_AndStats(t *testing.T) {
	g, err := config.ParseGenesis([]byte(`{
	  "config": {"chainId": 2205, "homesteadBlock": 0, "eip150Block": 0, "eip155Block": 0, "eip158Block": 0,
	    "byzantiumBlock": 0, "constantinopleBlock": 0, "petersburgBlock": 0, "istanbulBlock": 0,
	    "muirGlacierBlock": 0, "berlinBlock": 0, "londonBlock": 0, "shanghaiBlock": 0, "cancunBlock": 0},
	  "timestamp": 0, "extraData": "0x", "gasLimit": "0x7270e00", "baseFeePerGas": "0x3b9aca00",
	  "alloc": {
	    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {"balance": "1000000000000000000000000"}
	  }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	n := node.OpenTest(t, g)
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	st := rpcCall(t, srv, "dew_getExecutionStats", nil)
	m, ok := st.(map[string]interface{})
	if !ok {
		t.Fatalf("stats = %v", st)
	}
	if _, ok := m["conflict_rollback_rate"]; !ok {
		t.Fatal("missing conflict_rollback_rate")
	}

	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	dtx := types.NewDewTx(big.NewInt(2205), 0, sender, recv, uint256.NewInt(1000), params.DefaultDewTxFeeWei, nil, nil)
	if err := types.SignDewTx(dtx, key); err != nil {
		t.Fatal(err)
	}
	raw, err := dtx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	res := rpcCall(t, srv, "dew_sendRawTransaction", []interface{}{EncodeBytes(raw)})
	txHash, ok := res.(string)
	if !ok || len(txHash) != 66 {
		t.Fatalf("dew_sendRaw = %v", res)
	}
	if bn := rpcCall(t, srv, "eth_blockNumber", nil); bn != "0x1" {
		t.Fatalf("blockNumber = %v", bn)
	}
	bal := rpcCall(t, srv, "eth_getBalance", []interface{}{recv.Hex(), "latest"})
	if bal == "0x0" {
		t.Fatal("recipient still zero after DewTx")
	}
	_ = txHash

	// Feature flag off rejects
	n.SetNativeEnabled(false)
	body := map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "dew_sendRawTransaction",
		"params": []interface{}{EncodeBytes(raw)},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	var resp struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error when native disabled")
	}
}

func rpcCall(t *testing.T, srv *Server, method string, params interface{}) interface{} {
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
	if rr.Code != 200 {
		t.Fatalf("http %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Result interface{} `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("%s error: %s", method, resp.Error.Message)
	}
	return resp.Result
}

func TestEncodeChainIDPublicTestnet(t *testing.T) {
	if EncodeBig(big.NewInt(2205)) != "0x89d" {
		t.Fatal(EncodeBig(big.NewInt(2205)))
	}
	_ = fmt.Sprintf
}
