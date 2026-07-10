package devnet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type rpcClient struct {
	url string
	id  int
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *rpcClient) call(method string, params []interface{}) (json.RawMessage, error) {
	c.id++
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.id,
		"method":  method,
		"params":  params,
	})
	res, err := http.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var out rpcResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rpc decode: %w body=%s", err, string(raw))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("%s: %s", method, out.Error.Message)
	}
	return out.Result, nil
}

func (c *rpcClient) chainID() (*big.Int, error) {
	raw, err := c.call("eth_chainId", nil)
	if err != nil {
		return nil, err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return parseHexBig(s)
}

func (c *rpcClient) nonce(addr string) (uint64, error) {
	raw, err := c.call("eth_getTransactionCount", []interface{}{addr, "latest"})
	if err != nil {
		return 0, err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	v, err := parseHexBig(s)
	if err != nil {
		return 0, err
	}
	return v.Uint64(), nil
}

func (c *rpcClient) sendRaw(rawTx []byte) (string, error) {
	hexTx := "0x" + common.Bytes2Hex(rawTx)
	raw, err := c.call("eth_sendRawTransaction", []interface{}{hexTx})
	if err != nil {
		return "", err
	}
	var hash string
	if err := json.Unmarshal(raw, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

type receipt struct {
	Status          uint64
	ContractAddress common.Address
}

func (c *rpcClient) waitReceipt(txHash string) (*receipt, error) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := c.call("eth_getTransactionReceipt", []interface{}{txHash})
		if err != nil {
			return nil, err
		}
		if string(raw) == "null" {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		var obj struct {
			Status          string `json:"status"`
			ContractAddress string `json:"contractAddress"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, err
		}
		st, _ := parseHexBig(obj.Status)
		r := &receipt{Status: st.Uint64()}
		if obj.ContractAddress != "" && obj.ContractAddress != "0x" {
			r.ContractAddress = common.HexToAddress(obj.ContractAddress)
		}
		return r, nil
	}
	return nil, fmt.Errorf("timeout waiting receipt %s", txHash)
}

func (c *rpcClient) ethCall(to, data string) (string, error) {
	raw, err := c.call("eth_call", []interface{}{
		map[string]string{"to": to, "data": data},
		"latest",
	})
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func parseHexBig(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), nil
	}
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		s = "0x" + s
	}
	v, ok := new(big.Int).SetString(s, 0)
	if !ok {
		return nil, fmt.Errorf("invalid hex int %q", s)
	}
	return v, nil
}
