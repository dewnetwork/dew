// Package rpc implements the Ethereum-compatible JSON-RPC HTTP API for Dew.
package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Handler dispatches a single JSON-RPC method.
type Handler func(params json.RawMessage) (interface{}, error)

// Server is a JSON-RPC 2.0 HTTP server.
type Server struct {
	mu       sync.RWMutex
	methods  map[string]Handler
	listener net.Listener
	httpSrv  *http.Server
}

// NewServer creates an empty RPC server.
func NewServer() *Server {
	return &Server{methods: make(map[string]Handler)}
}

// Register adds a method handler.
func (s *Server) Register(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.methods[method] = h
}

// RegisterAll registers a map of handlers.
func (s *Server) RegisterAll(m map[string]Handler) {
	for k, v := range m {
		s.Register(k, v)
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      json.RawMessage `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	// Batch or single
	trim := strings.TrimSpace(string(body))
	if strings.HasPrefix(trim, "[") {
		var reqs []rpcRequest
		if err := json.Unmarshal(body, &reqs); err != nil {
			writeJSON(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			return
		}
		resps := make([]rpcResponse, len(reqs))
		for i, req := range reqs {
			resps[i] = s.handleOne(req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(resps)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	writeJSON(w, s.handleOne(req))
}

func (s *Server) handleOne(req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	s.mu.RLock()
	h, ok := s.methods[req.Method]
	s.mu.RUnlock()
	if !ok {
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
		return resp
	}
	result, err := h(req.Params)
	if err != nil {
		resp.Error = &rpcError{Code: -32000, Message: err.Error()}
		return resp
	}
	resp.Result = result
	return resp
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(v)
}

// ListenAndServe starts HTTP on addr (e.g. "127.0.0.1:8545").
func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.httpSrv = &http.Server{Handler: s}
	fmt.Printf("Dew JSON-RPC listening on http://%s\n", ln.Addr().String())
	return s.httpSrv.Serve(ln)
}

// Addr returns the listening address (after ListenAndServe starts).
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Close shuts down the HTTP server.
func (s *Server) Close() error {
	if s.httpSrv != nil {
		return s.httpSrv.Close()
	}
	return nil
}
