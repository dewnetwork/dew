// Package faucet is a production-oriented HTTP faucet for public-testnet-v1.
//
// It is ops-only: it signs native transfers via a funded key and posts them
// through public JSON-RPC. It does not import consensus, node, or mempool
// packages so it can be stopped independently of validators.
//
// Modes (Config.Mode):
//   - allowlist: recipient must appear in the allowlist file
//   - captcha:   require a verified captcha token (Turnstile or hCaptcha)
//   - dev:       rate limits only (local / private nets; never for public)
package faucet
