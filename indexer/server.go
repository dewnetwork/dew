package indexer

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxContractBodyBytes = 512 * 1024
	contractPOSTMax      = 10
	contractPOSTWindow   = 10 * time.Minute
)

// Server is the HTTP API for the explorer (history + P1f contract registry).
type Server struct {
	ix          *Indexer
	http        *http.Server
	ln          net.Listener
	verifyToken string

	rateMu   sync.Mutex
	rateByIP map[string][]time.Time
}

// NewServer binds the indexer HTTP API.
func NewServer(ix *Indexer, addr string) (*Server, error) {
	if addr == "" {
		addr = DefaultConfig().ListenAddr
	}
	s := &Server{
		ix:          ix,
		verifyToken: ix.cfg.VerifyToken,
		rateByIP:    make(map[string][]time.Time),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/status", s.handleStatus)
	mux.HandleFunc("/v1/address/", s.handleAddress)
	mux.HandleFunc("/v1/stats/volume", s.handleVolume)
	mux.HandleFunc("/v1/contract/", s.handleContract)
	s.http = &http.Server{
		Handler:      withCORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.ln = ln
	return s, nil
}

// Addr returns the listening address.
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Start serves until Close.
func (s *Server) Start() error {
	return s.http.Serve(s.ln)
}

// Close shuts down the HTTP server.
func (s *Server) Close() error {
	return s.http.Close()
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Dew-Verify-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st, err := s.ix.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// /v1/address/{addr}/txs or /transfers
	path := strings.TrimPrefix(r.URL.Path, "/v1/address/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path /v1/address/{addr}/txs|transfers"})
		return
	}
	addr := parts[0]
	if !validHexAddress(addr) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid address"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	switch parts[1] {
	case "txs":
		rows, err := s.ix.store.AddressTxs(addr, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []TxRow{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"address": normalizeAddr(addr),
			"txs":     rows,
			"limit":   clampLimit(limit),
			"offset":  offset,
		})
	case "transfers":
		rows, err := s.ix.store.AddressTransfers(addr, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []TransferRow{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"address":   normalizeAddr(addr),
			"transfers": rows,
			"limit":     clampLimit(limit),
			"offset":    offset,
		})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown resource"})
	}
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 50
	}
	return limit
}

func (s *Server) handleVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	var fromB, toB uint64
	if v := q.Get("from"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid from"})
			return
		}
		fromB = n
	}
	if v := q.Get("to"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid to"})
			return
		}
		toB = n
	} else {
		toB = s.ix.indexed.Load()
	}
	// Default: last ~14 days of blocks if from unset — use full indexed range.
	if q.Get("from") == "" {
		fromB = 0
	}
	if toB < fromB {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to < from"})
		return
	}
	points, err := s.ix.store.VolumeByDay(fromB, toB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if points == nil {
		points = []VolumePoint{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fromBlock": fromB,
		"toBlock":   toB,
		"points":    points,
	})
}

func validHexAddress(addr string) bool {
	if !strings.HasPrefix(strings.ToLower(addr), "0x") || len(addr) != 42 {
		return false
	}
	for _, c := range addr[2:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func (s *Server) handleContract(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/contract/")
	path = strings.Trim(path, "/")
	if path == "" || strings.Contains(path, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path /v1/contract/{addr}"})
		return
	}
	if !validHexAddress(path) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid address"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleContractGet(w, path)
	case http.MethodPost:
		s.handleContractPost(w, r, path)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleContractGet(w http.ResponseWriter, addr string) {
	rec, ok, err := s.ix.store.GetContract(addr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, contractResponse(rec))
}

type contractPostBody struct {
	Name     string          `json:"name"`
	ABI      json.RawMessage `json:"abi"`
	Source   string          `json:"source"`
	Compiler string          `json:"compiler"`
}

func (s *Server) handleContractPost(w http.ResponseWriter, r *http.Request, addr string) {
	if !s.authorizeVerify(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !s.allowPOST(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxContractBodyBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
		return
	}
	if len(body) > maxContractBodyBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body too large"})
		return
	}
	var req contractPostBody
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.ABI) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "abi required"})
		return
	}
	// Require ABI to be a JSON array.
	var abiArr []json.RawMessage
	if err := json.Unmarshal(req.ABI, &abiArr); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "abi must be a JSON array"})
		return
	}
	abiCompact, err := json.Marshal(abiArr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid abi"})
		return
	}
	if utf8.RuneCountInString(req.Name) > 128 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name too long"})
		return
	}
	if utf8.RuneCountInString(req.Compiler) > 64 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "compiler too long"})
		return
	}
	if err := s.ix.store.UpsertContract(ContractRecord{
		Address:  addr,
		Name:     req.Name,
		ABIJSON:  string(abiCompact),
		Source:   req.Source,
		Compiler: req.Compiler,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rec, ok, err := s.ix.store.GetContract(addr)
	if err != nil || !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store read after write"})
		return
	}
	writeJSON(w, http.StatusOK, contractResponse(rec))
}

func contractResponse(rec ContractRecord) map[string]interface{} {
	var abi interface{}
	if err := json.Unmarshal([]byte(rec.ABIJSON), &abi); err != nil {
		abi = []interface{}{}
	}
	out := map[string]interface{}{
		"address":   rec.Address,
		"abi":       abi,
		"status":    "registered",
		"createdAt": rec.CreatedAt,
		"updatedAt": rec.UpdatedAt,
	}
	if rec.Name != "" {
		out["name"] = rec.Name
	}
	if rec.Source != "" {
		out["source"] = rec.Source
	}
	if rec.Compiler != "" {
		out["compiler"] = rec.Compiler
	}
	return out
}

func (s *Server) authorizeVerify(r *http.Request) bool {
	if s.verifyToken == "" {
		return true
	}
	if h := r.Header.Get("X-Dew-Verify-Token"); h == s.verifyToken {
		return true
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == s.verifyToken
	}
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) allowPOST(ip string) bool {
	if ip == "" {
		ip = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-contractPOSTWindow)
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	times := s.rateByIP[ip]
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= contractPOSTMax {
		s.rateByIP[ip] = kept
		return false
	}
	s.rateByIP[ip] = append(kept, now)
	return true
}

// ListenURL is a helper for tests.
func (s *Server) ListenURL() string {
	return fmt.Sprintf("http://%s", s.Addr())
}
