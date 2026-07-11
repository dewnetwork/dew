package faucet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// mockChain is an in-memory Chain for unit tests.
type mockChain struct {
	mu      sync.Mutex
	chainID uint64
	nonce   uint64
	balance *big.Int
	sent    [][]byte
	fail    error
}

func (m *mockChain) ChainID(context.Context) (uint64, error) { return m.chainID, nil }

func (m *mockChain) Nonce(context.Context, string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nonce, nil
}

func (m *mockChain) BalanceWei(context.Context, string) (*big.Int, error) {
	return new(big.Int).Set(m.balance), nil
}

func (m *mockChain) SendRaw(_ context.Context, raw []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return "", m.fail
	}
	m.sent = append(m.sent, append([]byte(nil), raw...))
	m.nonce++
	return fmt.Sprintf("0x%064x", len(m.sent)), nil
}

func testKeyHex(t *testing.T) string {
	t.Helper()
	// Deterministic non-Anvil key for tests.
	return "1111111111111111111111111111111111111111111111111111111111111111"
}

func testServer(t *testing.T, mode Mode, allow map[string]struct{}, captcha CaptchaVerifier) (*Server, *mockChain) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Mode = mode
	cfg.PrivateKeyHex = testKeyHex(t)
	cfg.AllowAnvilKey = false
	if mode == ModeAllowlist {
		cfg.AllowlistPath = "/dev/null" // validated only; allow map passed separately
	}
	if mode == ModeCaptcha {
		cfg.CaptchaProvider = CaptchaTurnstile
		cfg.CaptchaSecret = "test-secret"
	}
	// Bypass path existence for allowlist mode in Validate by setting a real temp path.
	if mode == ModeAllowlist {
		// Validate only checks non-empty path, not file existence.
		cfg.AllowlistPath = "allowlist.txt"
	}
	chain := &mockChain{
		chainID: cfg.ChainID,
		balance: new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	}
	sender, err := NewSender(chain, cfg.PrivateKeyHex, cfg.ChainID, cfg.AmountWei, cfg.GasPriceWei, cfg.GasLimit)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(cfg, sender, captcha, allow)
	if err != nil {
		t.Fatal(err)
	}
	return srv, chain
}

func TestDripAllowlist(t *testing.T) {
	key, _ := ethcrypto.HexToECDSA(testKeyHex(t))
	// Use a fixed recipient
	to := "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	norm, _ := normalizeAddress(to)
	allow := map[string]struct{}{norm: {}}

	srv, chain := testServer(t, ModeAllowlist, allow, nil)

	body, _ := json.Marshal(map[string]string{"address": to})
	req := httptest.NewRequest(http.MethodPost, "/drip", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.1:1234"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(chain.sent) != 1 {
		t.Fatalf("sent=%d", len(chain.sent))
	}
	_ = key
}

func TestDripNotOnAllowlist(t *testing.T) {
	srv, _ := testServer(t, ModeAllowlist, map[string]struct{}{}, nil)
	body, _ := json.Marshal(map[string]string{"address": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"})
	req := httptest.NewRequest(http.MethodPost, "/drip", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.2:1234"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestDripRateLimitAddress(t *testing.T) {
	to := "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	norm, _ := normalizeAddress(to)
	srv, _ := testServer(t, ModeDev, nil, nil)
	// Override limits to 1 per address for clear assertion
	srv.addrLim = NewLimiter(1, srv.cfg.PerAddressWindow)
	srv.ipLim = NewLimiter(100, srv.cfg.PerIPWindow)

	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(map[string]string{"address": to})
		req := httptest.NewRequest(http.MethodPost, "/drip", bytes.NewReader(body))
		req.RemoteAddr = fmt.Sprintf("203.0.113.%d:1", i+10)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if i == 0 && rr.Code != http.StatusOK {
			t.Fatalf("first: %d %s", rr.Code, rr.Body.String())
		}
		if i == 1 && rr.Code != http.StatusTooManyRequests {
			t.Fatalf("second: %d %s", rr.Code, rr.Body.String())
		}
	}
	_ = norm
}

func TestDripCaptcha(t *testing.T) {
	srv, _ := testServer(t, ModeCaptcha, nil, StaticCaptcha{Accept: "good-token"})
	// missing token
	body, _ := json.Marshal(map[string]string{"address": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"})
	req := httptest.NewRequest(http.MethodPost, "/drip", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.50:1"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("missing captcha status=%d", rr.Code)
	}
	// good token
	body, _ = json.Marshal(map[string]string{
		"address":      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		"captchaToken": "good-token",
	})
	req = httptest.NewRequest(http.MethodPost, "/drip", bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.51:1"
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("captcha ok status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestInfoAndHealth(t *testing.T) {
	srv, _ := testServer(t, ModeDev, nil, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code)
	}
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/info", nil))
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code)
	}
	var info map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info["chainId"].(float64) != 2205 {
		t.Fatalf("chainId=%v", info["chainId"])
	}
}

func TestInvalidAddress(t *testing.T) {
	srv, _ := testServer(t, ModeDev, nil, nil)
	body, _ := json.Marshal(map[string]string{"address": "not-an-address"})
	req := httptest.NewRequest(http.MethodPost, "/drip", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:9"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}
