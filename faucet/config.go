package faucet

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/dewnetwork/dew/params"
)

// Mode selects admission policy before rate limits apply.
type Mode string

const (
	// ModeAllowlist requires the recipient address to be listed.
	ModeAllowlist Mode = "allowlist"
	// ModeCaptcha requires a verified captcha token (open mint with bot friction).
	ModeCaptcha Mode = "captcha"
	// ModeDev skips captcha/allowlist and applies rate limits only.
	// For private/local nets; refuse in production unless explicitly set.
	ModeDev Mode = "dev"
)

// CaptchaProvider identifies the remote captcha API.
type CaptchaProvider string

const (
	CaptchaNone      CaptchaProvider = ""
	CaptchaTurnstile CaptchaProvider = "turnstile"
	CaptchaHCaptcha  CaptchaProvider = "hcaptcha"
)

// AnvilAccount0PrivHex is Foundry Anvil account #0 — never use on public faucet.
const AnvilAccount0PrivHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// Default operator knobs for public-testnet-v1.
const (
	DefaultListenAddr        = "127.0.0.1:8080"
	DefaultRPCURL            = "http://127.0.0.1:8545"
	DefaultAmountDEW         = "1" // 1 DEW — deploy + a few transfers
	DefaultPerAddress        = 1
	DefaultPerAddressWindow  = 24 * time.Hour
	DefaultPerIP             = 10
	DefaultPerIPWindow       = time.Hour
	DefaultGasLimit          = uint64(21_000)
	DefaultGasPriceGwei      = int64(1) // 1 gwei floor (freeze)
	DefaultMaxRequestBody    = 8 << 10  // 8 KiB
)

// Config holds runtime settings for the faucet service.
type Config struct {
	// ListenAddr is HTTP bind address (default 127.0.0.1:8080 — put TLS proxy in front).
	ListenAddr string
	// RPCURL is the chain JSON-RPC endpoint.
	RPCURL string
	// ChainID expected eth_chainId (default params.PublicTestnetChainID).
	ChainID uint64
	// PrivateKeyHex is the funded faucet signer (0x-optional, 64 hex chars).
	PrivateKeyHex string
	// AmountWei is the fixed drip size.
	AmountWei *big.Int
	// Mode is allowlist | captcha | dev.
	Mode Mode
	// AllowlistPath is a newline-separated list of 0x addresses (required for ModeAllowlist).
	AllowlistPath string
	// CaptchaProvider is turnstile | hcaptcha when ModeCaptcha.
	CaptchaProvider CaptchaProvider
	// CaptchaSecret is the server-side captcha secret.
	CaptchaSecret string
	// PerAddress / PerAddressWindow rate-limit drips by recipient.
	PerAddress       int
	PerAddressWindow time.Duration
	// PerIP / PerIPWindow rate-limit drips by client IP.
	PerIP       int
	PerIPWindow time.Duration
	// GasLimit for native transfer (default 21000).
	GasLimit uint64
	// GasPriceWei for legacy txs (default 1 gwei).
	GasPriceWei *big.Int
	// AllowAnvilKey permits the Anvil #0 private key (dev only).
	AllowAnvilKey bool
	// MaxRequestBody bounds POST body size.
	MaxRequestBody int64
	// TrustedProxy when true trusts X-Forwarded-For first hop for IP limits.
	TrustedProxy bool
}

// DefaultConfig returns safe public-testnet defaults (keys and mode must still be set).
func DefaultConfig() Config {
	amount := new(big.Int).Mul(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	return Config{
		ListenAddr:       DefaultListenAddr,
		RPCURL:           DefaultRPCURL,
		ChainID:          params.PublicTestnetChainID,
		AmountWei:        amount,
		Mode:             ModeAllowlist,
		PerAddress:       DefaultPerAddress,
		PerAddressWindow: DefaultPerAddressWindow,
		PerIP:            DefaultPerIP,
		PerIPWindow:      DefaultPerIPWindow,
		GasLimit:         DefaultGasLimit,
		GasPriceWei:      big.NewInt(DefaultGasPriceGwei * 1_000_000_000),
		MaxRequestBody:   DefaultMaxRequestBody,
	}
}

// Validate checks config consistency before Start.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("faucet: nil config")
	}
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("faucet: listen address required")
	}
	if strings.TrimSpace(c.RPCURL) == "" {
		return fmt.Errorf("faucet: RPC URL required")
	}
	if c.ChainID == 0 {
		return fmt.Errorf("faucet: chain ID required")
	}
	if strings.TrimSpace(c.PrivateKeyHex) == "" {
		return fmt.Errorf("faucet: private key required")
	}
	if c.AmountWei == nil || c.AmountWei.Sign() <= 0 {
		return fmt.Errorf("faucet: amount must be positive")
	}
	if c.GasLimit == 0 {
		return fmt.Errorf("faucet: gas limit required")
	}
	if c.GasPriceWei == nil || c.GasPriceWei.Sign() <= 0 {
		return fmt.Errorf("faucet: gas price must be positive")
	}
	if c.PerAddress <= 0 || c.PerAddressWindow <= 0 {
		return fmt.Errorf("faucet: per-address rate limit must be positive")
	}
	if c.PerIP <= 0 || c.PerIPWindow <= 0 {
		return fmt.Errorf("faucet: per-IP rate limit must be positive")
	}
	if c.MaxRequestBody <= 0 {
		c.MaxRequestBody = DefaultMaxRequestBody
	}

	key := normalizePrivHex(c.PrivateKeyHex)
	if !c.AllowAnvilKey && strings.EqualFold(key, AnvilAccount0PrivHex) {
		return fmt.Errorf("faucet: Anvil #0 key refused (set AllowAnvilKey for local dev only)")
	}

	switch c.Mode {
	case ModeAllowlist:
		if strings.TrimSpace(c.AllowlistPath) == "" {
			return fmt.Errorf("faucet: allowlist mode requires AllowlistPath")
		}
	case ModeCaptcha:
		if c.CaptchaProvider != CaptchaTurnstile && c.CaptchaProvider != CaptchaHCaptcha {
			return fmt.Errorf("faucet: captcha mode requires provider turnstile or hcaptcha")
		}
		if strings.TrimSpace(c.CaptchaSecret) == "" {
			return fmt.Errorf("faucet: captcha mode requires CaptchaSecret")
		}
	case ModeDev:
		// ok
	default:
		return fmt.Errorf("faucet: unknown mode %q (allowlist|captcha|dev)", c.Mode)
	}
	return nil
}

// LoadAllowlistFile reads newline-separated addresses; # comments and blanks ignored.
func LoadAllowlistFile(path string) (map[string]struct{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("faucet: read allowlist: %w", err)
	}
	out := make(map[string]struct{})
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// optional trailing comment
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		addr, err := normalizeAddress(line)
		if err != nil {
			return nil, fmt.Errorf("faucet: allowlist entry %q: %w", line, err)
		}
		out[addr] = struct{}{}
	}
	return out, nil
}

func normalizePrivHex(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return strings.ToLower(s)
}
