package faucet

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidateAnvilKeyRefused(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PrivateKeyHex = AnvilAccount0PrivHex
	cfg.Mode = ModeDev
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected anvil key refused")
	}
	cfg.AllowAnvilKey = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("allow anvil: %v", err)
	}
}

func TestConfigValidateAllowlistRequiresPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PrivateKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.Mode = ModeAllowlist
	cfg.AllowlistPath = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected path required")
	}
}

func TestConfigValidateCaptcha(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PrivateKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg.Mode = ModeCaptcha
	cfg.CaptchaProvider = CaptchaTurnstile
	cfg.CaptchaSecret = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("secret required")
	}
	cfg.CaptchaSecret = "sec"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAllowlistFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "allow.txt")
	content := `# comment
0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
0x70997970C51812dc3A010C7d01b50e0d17dc79C8 # user1

`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadAllowlistFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("len=%d", len(m))
	}
	if _, ok := m["0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"]; !ok {
		t.Fatal("missing faucet addr")
	}
}

func TestDefaultAmountOneDEW(t *testing.T) {
	cfg := DefaultConfig()
	one := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	if cfg.AmountWei.Cmp(one) != 0 {
		t.Fatalf("amount %s", cfg.AmountWei)
	}
}
