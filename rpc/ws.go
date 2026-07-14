package rpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/node"
)

// WebSocket / subscription limits (abuse control; independent of HTTP body limits).
const (
	MaxWSSubscriptionsPerConn = 16
	MaxWSConnections          = 256
	wsWriteWait               = 10 * time.Second
	wsPongWait                = 60 * time.Second
	wsPingPeriod              = 30 * time.Second
	wsMaxMessageBytes         = MaxRequestBodyBytes
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // public RPC; CORS-open like HTTP
	},
}

// EnableSubscriptions wires Node chain events for eth_subscribe over WebSocket.
// Call after RegisterAll. Safe to call once; subsequent calls replace the backend.
func (s *Server) EnableSubscriptions(n *node.Node, api *API) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.node = n
	s.api = api
	s.wsEnabled = n != nil && api != nil
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	enabled := s.wsEnabled && s.node != nil && s.api != nil
	n := s.node
	api := s.api
	s.mu.RUnlock()
	if !enabled {
		http.Error(w, "websocket subscriptions not enabled", http.StatusServiceUnavailable)
		return
	}
	if atomic.LoadInt32(&s.wsConns) >= MaxWSConnections {
		http.Error(w, "too many websocket connections", http.StatusServiceUnavailable)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	atomic.AddInt32(&s.wsConns, 1)
	defer atomic.AddInt32(&s.wsConns, -1)

	c := &wsConn{
		server: s,
		conn:   conn,
		node:   n,
		api:    api,
		subs:   make(map[string]*wsSub),
		send:   make(chan []byte, 32),
	}
	events, unsubChain := n.SubscribeChainEvents()
	c.unsubChain = unsubChain
	defer c.close()

	go c.writePump()
	go c.eventPump(events)
	c.readPump()
}

type wsSubKind int

const (
	subNewHeads wsSubKind = iota
	subLogs
)

type wsSub struct {
	id      string
	kind    wsSubKind
	addrSet map[crypto.Address]struct{}
	topics  [][]types.Hash
}

type wsConn struct {
	server     *Server
	conn       *websocket.Conn
	node       *node.Node
	api        *API
	mu         sync.Mutex
	subs       map[string]*wsSub
	send       chan []byte
	unsubChain func()
	closed     atomic.Bool
}

func (c *wsConn) close() {
	if c.closed.Swap(true) {
		return
	}
	if c.unsubChain != nil {
		c.unsubChain()
	}
	c.mu.Lock()
	c.subs = nil
	c.mu.Unlock()
	_ = c.conn.Close()
}

func (c *wsConn) readPump() {
	defer c.close()
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetReadLimit(int64(wsMaxMessageBytes))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.handleMessage(data)
	}
}

func (c *wsConn) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if c.closed.Load() {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *wsConn) eventPump(events <-chan node.ChainEvent) {
	for ev := range events {
		if c.closed.Load() {
			return
		}
		c.dispatchEvent(ev)
	}
}

func (c *wsConn) enqueueJSON(v interface{}) {
	if c.closed.Load() {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
		// drop if client is slow
	}
}

func (c *wsConn) handleMessage(data []byte) {
	trim := strings.TrimSpace(string(data))
	if strings.HasPrefix(trim, "[") {
		var reqs []rpcRequest
		if err := json.Unmarshal(data, &reqs); err != nil {
			c.enqueueJSON(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			return
		}
		if len(reqs) > MaxBatchItems {
			c.enqueueJSON(rpcResponse{JSONRPC: "2.0", Error: &rpcError{
				Code:    -32600,
				Message: fmt.Sprintf("batch too large (max %d items)", MaxBatchItems),
			}})
			return
		}
		for _, req := range reqs {
			c.enqueueJSON(c.handleOne(req))
		}
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		c.enqueueJSON(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	c.enqueueJSON(c.handleOne(req))
}

func (c *wsConn) handleOne(req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "eth_subscribe":
		id, err := c.subscribe(req.Params)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = id
		return resp
	case "eth_unsubscribe":
		ok, err := c.unsubscribe(req.Params)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = ok
		return resp
	default:
		return c.server.handleOne(req)
	}
}

func (c *wsConn) subscribe(params json.RawMessage) (string, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(params, &raw); err != nil || len(raw) < 1 {
		return "", fmt.Errorf("invalid params")
	}
	var name string
	if err := json.Unmarshal(raw[0], &name); err != nil {
		return "", fmt.Errorf("invalid subscription name")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subs == nil {
		return "", fmt.Errorf("connection closed")
	}
	if len(c.subs) >= MaxWSSubscriptionsPerConn {
		return "", fmt.Errorf("too many subscriptions (max %d)", MaxWSSubscriptionsPerConn)
	}

	sub := &wsSub{id: newSubscriptionID()}
	switch name {
	case "newHeads":
		sub.kind = subNewHeads
	case "logs":
		sub.kind = subLogs
		if len(raw) > 1 {
			addrSet, topics, err := parseLogsFilter(raw[1])
			if err != nil {
				return "", err
			}
			sub.addrSet = addrSet
			sub.topics = topics
		}
	default:
		return "", fmt.Errorf("unsupported subscription: %s", name)
	}
	c.subs[sub.id] = sub
	return sub.id, nil
}

func (c *wsConn) unsubscribe(params json.RawMessage) (bool, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return false, fmt.Errorf("invalid params")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subs == nil {
		return false, nil
	}
	if _, ok := c.subs[p[0]]; !ok {
		return false, nil
	}
	delete(c.subs, p[0])
	return true, nil
}

func (c *wsConn) dispatchEvent(ev node.ChainEvent) {
	c.mu.Lock()
	subs := make([]*wsSub, 0, len(c.subs))
	for _, s := range c.subs {
		subs = append(subs, s)
	}
	c.mu.Unlock()

	for _, s := range subs {
		switch s.kind {
		case subNewHeads:
			if ev.Header == nil {
				continue
			}
			result := c.api.formatHeader(ev.Header, ev.Hash)
			c.enqueueJSON(subscriptionNote{
				JSONRPC: "2.0",
				Method:  "eth_subscription",
				Params: subscriptionParams{
					Subscription: s.id,
					Result:       result,
				},
			})
		case subLogs:
			for _, il := range ev.Logs {
				if il == nil || il.Log == nil {
					continue
				}
				if !matchLogSub(il, s.addrSet, s.topics) {
					continue
				}
				c.enqueueJSON(subscriptionNote{
					JSONRPC: "2.0",
					Method:  "eth_subscription",
					Params: subscriptionParams{
						Subscription: s.id,
						Result:       formatIndexedLog(il),
					},
				})
			}
		}
	}
}

type subscriptionNote struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  subscriptionParams `json:"params"`
}

type subscriptionParams struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result"`
}

func newSubscriptionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// extremely unlikely; fall back to timestamp-ish zeros still unique in map rarely
	}
	return EncodeBytes(b[:])
}

func parseLogsFilter(raw json.RawMessage) (map[crypto.Address]struct{}, [][]types.Hash, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, nil, fmt.Errorf("invalid logs filter")
	}
	addrSet := map[crypto.Address]struct{}{}
	if v, ok := obj["address"]; ok {
		var one string
		if err := json.Unmarshal(v, &one); err == nil && one != "" {
			a, err := DecodeAddress(one)
			if err != nil {
				return nil, nil, err
			}
			addrSet[a] = struct{}{}
		} else {
			var many []string
			if err := json.Unmarshal(v, &many); err != nil {
				return nil, nil, fmt.Errorf("invalid address filter")
			}
			for _, s := range many {
				a, err := DecodeAddress(s)
				if err != nil {
					return nil, nil, err
				}
				addrSet[a] = struct{}{}
			}
		}
	}
	var topics [][]types.Hash
	if v, ok := obj["topics"]; ok && string(v) != "null" {
		var rawTopics []json.RawMessage
		if err := json.Unmarshal(v, &rawTopics); err != nil {
			return nil, nil, fmt.Errorf("invalid topics filter")
		}
		topics = make([][]types.Hash, len(rawTopics))
		for i, rt := range rawTopics {
			if string(rt) == "null" {
				continue
			}
			var one string
			if err := json.Unmarshal(rt, &one); err == nil {
				if one == "" {
					continue
				}
				h, err := DecodeHash(one)
				if err != nil {
					return nil, nil, err
				}
				topics[i] = []types.Hash{h}
				continue
			}
			var many []string
			if err := json.Unmarshal(rt, &many); err != nil {
				return nil, nil, fmt.Errorf("invalid topics filter")
			}
			alts := make([]types.Hash, 0, len(many))
			for _, s := range many {
				if s == "" {
					continue
				}
				h, err := DecodeHash(s)
				if err != nil {
					return nil, nil, err
				}
				alts = append(alts, h)
			}
			topics[i] = alts
		}
	}
	return addrSet, topics, nil
}

func matchLogSub(il *node.IndexedLog, addrSet map[crypto.Address]struct{}, topics [][]types.Hash) bool {
	if len(addrSet) > 0 {
		if _, ok := addrSet[il.Log.Address]; !ok {
			return false
		}
	}
	if len(topics) == 0 {
		return true
	}
	for i, alts := range topics {
		if len(alts) == 0 {
			continue
		}
		if i >= len(il.Log.Topics) {
			return false
		}
		ok := false
		want := il.Log.Topics[i]
		for _, t := range alts {
			if t == want {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func formatIndexedLog(il *node.IndexedLog) map[string]interface{} {
	topics := make([]string, len(il.Log.Topics))
	for i, t := range il.Log.Topics {
		topics[i] = EncodeHash(t)
	}
	return map[string]interface{}{
		"address":          EncodeAddress(il.Log.Address),
		"topics":           topics,
		"data":             EncodeBytes(il.Log.Data),
		"blockNumber":      EncodeUint64(il.BlockNumber),
		"transactionHash":  EncodeHash(il.TxHash),
		"transactionIndex": EncodeUint64(uint64(il.TxIndex)),
		"blockHash":        EncodeHash(il.BlockHash),
		"logIndex":         EncodeUint64(uint64(il.Index)),
		"removed":          false,
	}
}
