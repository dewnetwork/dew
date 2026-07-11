package faucet

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// Server is the HTTP faucet API.
type Server struct {
	cfg      Config
	sender   *Sender
	addrLim  *Limiter
	ipLim    *Limiter
	allow    map[string]struct{}
	captcha  CaptchaVerifier
	httpSrv  *http.Server
	logf     func(format string, args ...interface{})
}

// NewServer constructs an HTTP faucet. allowlist may be empty for captcha/dev modes.
func NewServer(cfg Config, sender *Sender, captcha CaptchaVerifier, allowlist map[string]struct{}) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, fmt.Errorf("faucet: sender required")
	}
	if allowlist == nil {
		allowlist = map[string]struct{}{}
	}
	if cfg.Mode == ModeCaptcha && captcha == nil {
		return nil, fmt.Errorf("faucet: captcha verifier required in captcha mode")
	}
	s := &Server{
		cfg:     cfg,
		sender:  sender,
		addrLim: NewLimiter(cfg.PerAddress, cfg.PerAddressWindow),
		ipLim:   NewLimiter(cfg.PerIP, cfg.PerIPWindow),
		allow:   allowlist,
		captcha: captcha,
		logf:    log.Printf,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /info", s.handleInfo)
	mux.HandleFunc("POST /drip", s.handleDrip)
	// Go 1.21 fallback patterns without method (for older tooling): also register bare paths
	// Method-specific patterns require Go 1.22+; module is 1.23 so OK.
	s.httpSrv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.withLimits(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 14,
	}
	return s, nil
}

func (s *Server) withLimits(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxRequestBody)
		}
		// CORS: simple public faucet UI may be separate origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler returns the HTTP handler (for tests / custom Listen).
func (s *Server) Handler() http.Handler {
	return s.httpSrv.Handler
}

// Start listens and serves until Shutdown.
func (s *Server) Start() error {
	s.logf("faucet listening on %s mode=%s chainId=%d amountWei=%s from=%s",
		s.cfg.ListenAddr, s.cfg.Mode, s.cfg.ChainID, s.cfg.AmountWei.String(), s.sender.From().Hex())
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"chainId":            s.cfg.ChainID,
		"mode":               string(s.cfg.Mode),
		"amountWei":          s.cfg.AmountWei.String(),
		"from":               s.sender.From().Hex(),
		"perAddress":         s.cfg.PerAddress,
		"perAddressWindowSec": int(s.cfg.PerAddressWindow.Seconds()),
		"perIP":              s.cfg.PerIP,
		"perIPWindowSec":     int(s.cfg.PerIPWindow.Seconds()),
		"freezeTag":          "public-testnet-v1",
	})
}

type dripRequest struct {
	Address string `json:"address"`
	// CaptchaToken is required in captcha mode (Turnstile/hCaptcha response).
	CaptchaToken string `json:"captchaToken"`
}

func (s *Server) handleDrip(w http.ResponseWriter, r *http.Request) {
	var req dripRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	addr, err := normalizeAddress(req.Address)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid address")
		return
	}

	ip := clientIP(r, s.cfg.TrustedProxy)

	switch s.cfg.Mode {
	case ModeAllowlist:
		if _, ok := s.allow[addr]; !ok {
			writeErr(w, http.StatusForbidden, "address not on allowlist")
			return
		}
	case ModeCaptcha:
		if s.captcha == nil {
			writeErr(w, http.StatusInternalServerError, "captcha not configured")
			return
		}
		if err := s.captcha.Verify(r.Context(), req.CaptchaToken, ip); err != nil {
			writeErr(w, http.StatusForbidden, "captcha verification failed")
			return
		}
	case ModeDev:
		// rate limits only
	}

	if !s.ipLim.Allow(ip) {
		writeErr(w, http.StatusTooManyRequests, "rate limit: IP")
		return
	}
	if !s.addrLim.Allow(addr) {
		writeErr(w, http.StatusTooManyRequests, "rate limit: address")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	res, err := s.sender.Drip(ctx, addr)
	if err != nil {
		s.logf("drip error ip=%s to=%s: %v", ip, addr, err)
		// Do not leak internal detail heavily; still useful for operators in logs.
		msg := "transfer failed"
		if strings.Contains(err.Error(), "underfunded") {
			msg = "faucet underfunded"
		} else if strings.Contains(err.Error(), "wrong chain") {
			msg = "wrong chain id"
		} else if strings.Contains(err.Error(), "invalid recipient") {
			msg = "invalid address"
		}
		status := http.StatusBadGateway
		if strings.Contains(msg, "underfunded") {
			status = http.StatusServiceUnavailable
		}
		writeErr(w, status, msg)
		return
	}
	s.logf("drip ok ip=%s to=%s tx=%s", ip, addr, res.TxHash)
	writeJSON(w, http.StatusOK, map[string]string{
		"txHash": res.TxHash,
		"from":   res.From,
		"to":     res.To,
		"amount": res.Amount,
	})
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
