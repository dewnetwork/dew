package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type rpcClient struct {
	url    string
	client *http.Client
	id     int
}

func newRPCClient(url string) *rpcClient {
	return &rpcClient{
		url: strings.TrimRight(url, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *rpcClient) call(method string, params []interface{}) (json.RawMessage, error) {
	c.id++
	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	res, err := c.client.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var out struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rpc decode: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("%s: %s", method, out.Error.Message)
	}
	return out.Result, nil
}

func (c *rpcClient) blockNumber() (uint64, error) {
	raw, err := c.call("eth_blockNumber", nil)
	if err != nil {
		return 0, err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	return parseHexUint64(s)
}

func (c *rpcClient) chainID() (string, error) {
	raw, err := c.call("eth_chainId", nil)
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

type rpcBlock struct {
	Number       string          `json:"number"`
	Hash         string          `json:"hash"`
	Timestamp    string          `json:"timestamp"`
	GasUsed      string          `json:"gasUsed"`
	Transactions json.RawMessage `json:"transactions"`
}

type rpcTx struct {
	Hash  string  `json:"hash"`
	From  string  `json:"from"`
	To    *string `json:"to"`
	Value string  `json:"value"`
	Index string  `json:"transactionIndex"`
}

type rpcReceipt struct {
	Status            string    `json:"status"`
	GasUsed           string    `json:"gasUsed"`
	TransactionHash   string    `json:"transactionHash"`
	TransactionIndex  string    `json:"transactionIndex"`
	BlockNumber       string    `json:"blockNumber"`
	Logs              []rpcLog  `json:"logs"`
	From              string    `json:"from"`
	To                *string   `json:"to"`
	ContractAddress   *string   `json:"contractAddress"`
}

type rpcLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	LogIndex         string   `json:"logIndex"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockNumber      string   `json:"blockNumber"`
}

func (c *rpcClient) getBlock(num uint64, full bool) (*rpcBlock, error) {
	raw, err := c.call("eth_getBlockByNumber", []interface{}{encodeUint64(num), full})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var b rpcBlock
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (c *rpcClient) getReceipt(txHash string) (*rpcReceipt, error) {
	raw, err := c.call("eth_getTransactionReceipt", []interface{}{txHash})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var r rpcReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func parseHexUint64(s string) (uint64, error) {
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if s == "" {
		return 0, nil
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, 16); !ok {
		return 0, fmt.Errorf("invalid hex uint: %s", s)
	}
	return n.Uint64(), nil
}

func encodeUint64(n uint64) string {
	return fmt.Sprintf("0x%x", n)
}

func topicWordToAddr(topic string) string {
	// 32-byte topic → last 20 bytes as address
	h := strings.TrimPrefix(strings.ToLower(topic), "0x")
	if len(h) < 40 {
		return "0x" + h
	}
	return "0x" + h[len(h)-40:]
}
