package indexer

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Server is the read-only HTTP API for the explorer.
type Server struct {
	ix     *Indexer
	http   *http.Server
	ln     net.Listener
}

// NewServer binds the indexer HTTP API.
func NewServer(ix *Indexer, addr string) (*Server, error) {
	if addr == "" {
		addr = DefaultConfig().ListenAddr
	}
	s := &Server{ix: ix}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/status", s.handleStatus)
	mux.HandleFunc("/v1/address/", s.handleAddress)
	mux.HandleFunc("/v1/stats/volume", s.handleVolume)
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
	if !strings.HasPrefix(strings.ToLower(addr), "0x") || len(addr) != 42 {
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

// ListenURL is a helper for tests.
func (s *Server) ListenURL() string {
	return fmt.Sprintf("http://%s", s.Addr())
}
