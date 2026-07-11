// Command dewfaucet runs the Phase D2 production faucet HTTP service.
//
// Ops-only: stop independently of validators. Never use Anvil keys on public nets.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dewnetwork/dew/faucet"
	"github.com/dewnetwork/dew/params"
)

func main() {
	cfg := faucet.DefaultConfig()

	listen := flag.String("http.addr", envOr("FAUCET_LISTEN", cfg.ListenAddr), "HTTP listen address")
	rpcURL := flag.String("rpc", envOr("FAUCET_RPC_URL", cfg.RPCURL), "JSON-RPC URL")
	chainID := flag.Uint64("chain-id", envUint64("FAUCET_CHAIN_ID", cfg.ChainID), "expected chain ID")
	mode := flag.String("mode", envOr("FAUCET_MODE", string(cfg.Mode)), "allowlist | captcha | dev")
	priv := flag.String("key", envOr("FAUCET_PRIVATE_KEY", ""), "funded faucet private key hex (or FAUCET_PRIVATE_KEY)")
	amountWei := flag.String("amount-wei", envOr("FAUCET_AMOUNT_WEI", cfg.AmountWei.String()), "drip amount in wei")
	allowPath := flag.String("allowlist", envOr("FAUCET_ALLOWLIST", ""), "path to allowlist file (mode=allowlist)")
	captchaProv := flag.String("captcha.provider", envOr("FAUCET_CAPTCHA_PROVIDER", ""), "turnstile | hcaptcha")
	captchaSecret := flag.String("captcha.secret", envOr("FAUCET_CAPTCHA_SECRET", ""), "captcha site secret")
	perAddr := flag.Int("limit.address", envInt("FAUCET_LIMIT_ADDRESS", cfg.PerAddress), "max drips per address per window")
	perAddrWin := flag.Duration("limit.address-window", envDuration("FAUCET_LIMIT_ADDRESS_WINDOW", cfg.PerAddressWindow), "address rate window")
	perIP := flag.Int("limit.ip", envInt("FAUCET_LIMIT_IP", cfg.PerIP), "max drips per IP per window")
	perIPWin := flag.Duration("limit.ip-window", envDuration("FAUCET_LIMIT_IP_WINDOW", cfg.PerIPWindow), "IP rate window")
	allowAnvil := flag.Bool("allow-anvil-key", envBool("FAUCET_ALLOW_ANVIL_KEY", false), "permit Anvil #0 key (local only)")
	trustedProxy := flag.Bool("trusted-proxy", envBool("FAUCET_TRUSTED_PROXY", false), "trust X-Forwarded-For for IP limits")
	flag.Parse()

	amount, ok := new(big.Int).SetString(strings.TrimSpace(*amountWei), 10)
	if !ok || amount.Sign() <= 0 {
		log.Fatalf("invalid -amount-wei %q", *amountWei)
	}

	cfg.ListenAddr = *listen
	cfg.RPCURL = *rpcURL
	cfg.ChainID = *chainID
	cfg.Mode = faucet.Mode(strings.ToLower(strings.TrimSpace(*mode)))
	cfg.PrivateKeyHex = strings.TrimSpace(*priv)
	cfg.AmountWei = amount
	cfg.AllowlistPath = *allowPath
	cfg.CaptchaProvider = faucet.CaptchaProvider(strings.ToLower(strings.TrimSpace(*captchaProv)))
	cfg.CaptchaSecret = *captchaSecret
	cfg.PerAddress = *perAddr
	cfg.PerAddressWindow = *perAddrWin
	cfg.PerIP = *perIP
	cfg.PerIPWindow = *perIPWin
	cfg.AllowAnvilKey = *allowAnvil
	cfg.TrustedProxy = *trustedProxy

	if cfg.ChainID == 0 {
		cfg.ChainID = params.PublicTestnetChainID
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	var allow map[string]struct{}
	if cfg.Mode == faucet.ModeAllowlist {
		var err error
		allow, err = faucet.LoadAllowlistFile(cfg.AllowlistPath)
		if err != nil {
			log.Fatal(err)
		}
		if len(allow) == 0 {
			log.Fatal("faucet: allowlist is empty")
		}
		log.Printf("loaded allowlist entries=%d from %s", len(allow), cfg.AllowlistPath)
	}

	var captcha faucet.CaptchaVerifier
	if cfg.Mode == faucet.ModeCaptcha {
		captcha = &faucet.HTTPCaptchaVerifier{
			Provider: cfg.CaptchaProvider,
			Secret:   cfg.CaptchaSecret,
		}
	}

	rpc := &faucet.RPCClient{URL: cfg.RPCURL}
	sender, err := faucet.NewSender(rpc, cfg.PrivateKeyHex, cfg.ChainID, cfg.AmountWei, cfg.GasPriceWei, cfg.GasLimit)
	if err != nil {
		log.Fatal(err)
	}

	// Startup chain-id probe (fail fast if RPC wrong).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	got, err := rpc.ChainID(ctx)
	cancel()
	if err != nil {
		log.Fatalf("rpc chain id: %v", err)
	}
	if got != cfg.ChainID {
		log.Fatalf("rpc chain id %d != configured %d", got, cfg.ChainID)
	}

	srv, err := faucet.NewServer(cfg, sender, captcha, allow)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
	fmt.Println("faucet stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envUint64(key string, def uint64) uint64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n uint64
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		return def
	}
	return n
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}
