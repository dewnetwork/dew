---
title: Production faucet
description: Phase D2 ops faucet — rate limits, allowlist/captcha, dewfaucet CLI, publish template.
category: development
order: 53
status: draft
---

# Production faucet (Phase D2)

The **faucet** is an ops HTTP service that drips a **small fixed amount** of test DEW to user addresses. It is **not** part of the consensus wire freeze: stop it independently of validators.

| Item | Status |
| :--- | :--- |
| In-repo service | **Shipped** — `faucet/` + `cmd/dewfaucet` |
| Wire / genesis impact | **None** — signs normal EVM value transfers via JSON-RPC |
| Public publish field `Faucet:` | URL, or `none` / `allowlist only` |

Policy baseline: [Public testnet freeze](./public-testnet.md) · Launch: [Launch checklist](./launch-checklist.md).

## Design

```text
Client ──POST /drip──▶ dewfaucet ──eth_sendRawTransaction──▶ Dew JSON-RPC
                         │
                         ├─ mode: allowlist | captcha | dev
                         ├─ rate limit: per IP + per address
                         └─ funded EOA key (env / EnvironmentFile only)
```

Rules:

- **Never** use Anvil / Foundry default keys on a public faucet (`AllowAnvilKey` is local-only).
- **Never** embed private keys in the explorer, docs site, or git.
- Prefer bind **`127.0.0.1`** and put TLS + rate limiting on nginx/Caddy (same pattern as path B RPC).
- Default drip: **1 DEW** (`1e18` wei) — enough for deploy + a few transfers, not economic yield.

## Modes

| Mode | When | Admission |
| :--- | :--- | :--- |
| **`allowlist`** (default public) | Controlled invites / form-approved addresses | Recipient must appear in allowlist file |
| **`captcha`** | Open mint with bot friction | Cloudflare Turnstile or hCaptcha token verified server-side |
| **`dev`** | Local / private only | Rate limits only — do **not** expose publicly |

Public operators must use **allowlist or captcha** (D2 acceptance). `dev` is for private nets and CI.

## Operator defaults (rate limits)

| Limit | Default | Flag / env |
| :--- | :--- | :--- |
| Per address | **1** drip / **24h** | `-limit.address` / `FAUCET_LIMIT_ADDRESS` + window |
| Per IP | **10** drips / **1h** | `-limit.ip` / `FAUCET_LIMIT_IP` + window |
| Amount | **1 DEW** | `-amount-wei` / `FAUCET_AMOUNT_WEI` |
| Gas price | **1 gwei** (legacy) | code default (`params` freeze floor) |
| Listen | `127.0.0.1:8080` | `-http.addr` / `FAUCET_LISTEN` |
| Chain ID | **2205** | `-chain-id` / `FAUCET_CHAIN_ID` |

Document the live numbers on the launch page when they differ.

## HTTP API

### `GET /health`

```json
{ "status": "ok" }
```

### `GET /info`

Public metadata (no secrets):

```json
{
  "chainId": 2205,
  "mode": "allowlist",
  "amountWei": "1000000000000000000",
  "from": "0x…",
  "perAddress": 1,
  "perAddressWindowSec": 86400,
  "perIP": 10,
  "perIPWindowSec": 3600,
  "freezeTag": "public-testnet-v1"
}
```

### `POST /drip`

```json
{
  "address": "0xRecipient…",
  "captchaToken": "…"
}
```

`captchaToken` required only in **captcha** mode.

| Status | Meaning |
| ---: | :--- |
| 200 | `{ "txHash", "from", "to", "amount" }` |
| 400 | Invalid JSON / address |
| 403 | Not allowlisted / captcha failed |
| 429 | Rate limited (IP or address) |
| 502 / 503 | RPC / underfunded faucet |

CORS is open (`*`) so a separate static drip page can call the API.

## Run locally (against `dew devnet`)

```bash
# Terminal 1
go run ./cmd/dew run --genesis genesis.json --http.port 8545
# or: go run ./cmd/dew devnet

# Terminal 2 — dev mode + Anvil #0 (LOCAL ONLY)
go run ./cmd/dewfaucet \
  -mode dev \
  -allow-anvil-key \
  -rpc http://127.0.0.1:8545 \
  -key ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
  -http.addr 127.0.0.1:8080

curl -s http://127.0.0.1:8080/info | jq .
curl -s -X POST http://127.0.0.1:8080/drip \
  -H 'content-type: application/json' \
  -d '{"address":"0x70997970C51812dc3A010C7d01b50e0d17dc79C8"}'
```

Allowlist mode:

```bash
cp faucet/allowlist.example.txt /tmp/allow.txt
# edit /tmp/allow.txt
go run ./cmd/dewfaucet \
  -mode allowlist \
  -allowlist /tmp/allow.txt \
  -rpc http://127.0.0.1:8545 \
  -key <funded-hex> \
  -http.addr 127.0.0.1:8080
```

Build:

```bash
go build -o bin/dewfaucet ./cmd/dewfaucet
```

## Production packaging

1. Fund a **new** EOA in genesis `alloc` (or top up offline) — never Anvil #0.
2. `cp deploy/faucet/faucet.env.example /etc/dew/faucet.env` and set secrets (`chmod 600`).
3. Install allowlist: `/etc/dew/faucet-allowlist.txt`.
4. Install binary + unit:

```bash
sudo install -m 755 bin/dewfaucet /usr/local/bin/dewfaucet
sudo install -m 644 deploy/faucet/systemd/dewfaucet.service /etc/systemd/system/dewfaucet.service
sudo systemctl daemon-reload
sudo systemctl enable --now dewfaucet
```

5. Reverse-proxy `127.0.0.1:8080` with TLS; set `FAUCET_TRUSTED_PROXY=true` if you rely on `X-Forwarded-For` for IP limits.
6. Emergency: `systemctl stop dewfaucet` — validators/RPC can stay up.

Env sample: [faucet.env.example](../../deploy/faucet/faucet.env.example). Unit: [dewfaucet.service](../../deploy/faucet/systemd/dewfaucet.service).

## Publish template

```text
Network:     Dew public-testnet-v1
Chain ID:    2205
RPC:         https://rpc-dew.fadosoft.com
Symbol:      DEW
Explorer:    https://explorer-dew.fadosoft.com
Faucet:      https://faucet-dew.fadosoft.com   # or: none / allowlist only
             # rules: 1 DEW / address / 24h · 10 / IP / hour · captcha|allowlist
Bootnodes:   …
```

## Packages

| Path | Role |
| :--- | :--- |
| `faucet/` | Rate limit, captcha, RPC client, HTTP API (library) |
| `cmd/dewfaucet` | Process entry |
| `faucet/allowlist.example.txt` | Allowlist format sample |
| `faucet-web/` | React SPA drip UI — shares the landing design system (`web/`: ink/cyan tokens, Syne + Figtree) |
| `deploy/faucet/faucet.env.example` | Operator env |
| `deploy/faucet/systemd/dewfaucet.service` | systemd unit |

Tests: `go test ./faucet/`.

## Related

- [Phases](./phases.md) — D2 acceptance  
- [Public testnet freeze](./public-testnet.md)  
- [Launch checklist](./launch-checklist.md)  
- [Block explorer](./block-explorer.md) — never holds faucet keys  
