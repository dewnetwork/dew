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

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/node"
)

func TestRPC_ChainIdAndBalance(t *testing.T) {
	g, err := config.LoadGenesisFile("../genesis.json")
	if err != nil {
		// fallback parse sample
		g, err = config.ParseGenesis([]byte(`{
		  "config": {"chainId": 2026, "homesteadBlock": 0, "eip150Block": 0, "eip155Block": 0, "eip158Block": 0,
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
	n, err := node.NewFromGenesis(g)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	// eth_chainId
	res := rpcCall(t, srv, "eth_chainId", nil)
	if res != "0x7ea" { // 2026
		t.Fatalf("chainId = %v, want 0x7ea", res)
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
	if res != "2026" {
		t.Fatalf("net_version = %v", res)
	}
}

func TestRPC_SendRawTransaction_Transfer(t *testing.T) {
	g, err := config.ParseGenesis([]byte(`{
	  "config": {"chainId": 2026, "homesteadBlock": 0, "eip150Block": 0, "eip155Block": 0, "eip158Block": 0,
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
	n, err := node.NewFromGenesis(g)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	// Anvil key #0
	key, err := ethcrypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	from := ethcrypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")

	signer := ethtypes.LatestSignerForChainID(big.NewInt(2026))
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(2026),
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

func TestEncodeChainID2026(t *testing.T) {
	if EncodeBig(big.NewInt(2026)) != "0x7ea" {
		t.Fatal(EncodeBig(big.NewInt(2026)))
	}
	_ = fmt.Sprintf
}
