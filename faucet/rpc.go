package faucet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// RPCClient is a minimal eth_* JSON-RPC client for faucet transfers.
type RPCClient struct {
	URL        string
	HTTPClient *http.Client
	id         int
}

func (c *RPCClient) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  []interface{}   `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *RPCClient) call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	c.id++
	body, err := json.Marshal(rpcEnvelope{
		JSONRPC: "2.0",
		ID:      c.id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var out rpcEnvelope
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rpc decode: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("%s: %s", method, out.Error.Message)
	}
	return out.Result, nil
}

// ChainID returns eth_chainId as uint64.
func (c *RPCClient) ChainID(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "eth_chainId", nil)
	if err != nil {
		return 0, err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	n, err := parseHexUint64(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Nonce returns eth_getTransactionCount(address, "pending").
func (c *RPCClient) Nonce(ctx context.Context, addr string) (uint64, error) {
	raw, err := c.call(ctx, "eth_getTransactionCount", []interface{}{addr, "pending"})
	if err != nil {
		return 0, err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	return parseHexUint64(s)
}

// BalanceWei returns eth_getBalance(address, "latest").
func (c *RPCClient) BalanceWei(ctx context.Context, addr string) (*big.Int, error) {
	raw, err := c.call(ctx, "eth_getBalance", []interface{}{addr, "latest"})
	if err != nil {
		return nil, err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return parseHexBig(s)
}

// SendRaw posts eth_sendRawTransaction and returns the tx hash hex.
func (c *RPCClient) SendRaw(ctx context.Context, rawTx []byte) (string, error) {
	hexTx := "0x" + common.Bytes2Hex(rawTx)
	raw, err := c.call(ctx, "eth_sendRawTransaction", []interface{}{hexTx})
	if err != nil {
		return "", err
	}
	var hash string
	if err := json.Unmarshal(raw, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

func parseHexUint64(s string) (uint64, error) {
	n, err := parseHexBig(s)
	if err != nil {
		return 0, err
	}
	if !n.IsUint64() {
		return 0, fmt.Errorf("value too large")
	}
	return n.Uint64(), nil
}

func parseHexBig(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), nil
	}
	n, ok := new(big.Int).SetString(s, 0)
	if !ok {
		return nil, fmt.Errorf("invalid hex quantity %q", s)
	}
	return n, nil
}
