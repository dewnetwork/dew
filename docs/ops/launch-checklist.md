---
title: Launch checklist
description: One-page private soak and public-testnet-v1 operator checklist.
category: ops
order: 55
status: stable
---

# Launch checklist

Freeze tag: **`public-testnet-v1`** · chain ID **`2205`**.  
Packaging: [deploy/](../../deploy/) · freeze table: [Public testnet](./public-testnet.md) · private ops: [Private testnet](./private-testnet.md).

## Live deployment

Path B (**single-host controlled RPC**) is **live** (July 2026). Use the [publish template](#public-publish-template-path-b) for MetaMask, faucet pages, and builder docs. Path A (multi-host validators + bootnodes) is [D3](../build/phases.md#d3--scale-when-needed-path-a--c4--audit) — design spec: [D3 scale](../scale/d3-scale.md#d3d--path-a-multi-host-public).

```bash
node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com
# chainId 0x89d (2205) · net_version 2205
```

## A. Private staging (do this first)

| # | Step | Command / value |
| -: | :--- | :-------------- |
| 1 | Build binary | `go build -o bin/dew ./cmd/dew` |
| 2 | Chaos smoke | `go test ./devnet/ -run Chaos -count=1` |
| 3 | RPC abuse bar | `go test ./tests/security/ -run C6 -count=1` |
| 4 | Start private stack | **Compose:** `cp deploy/soak.env.example deploy/soak.env` then `docker compose -f deploy/docker-compose.soak.yml --env-file deploy/soak.env up --build` **or** `./bin/dew devnet --http.addr 0.0.0.0 --http.port 8545` |
| 5 | RPC smoke | `node scripts/smoke-rpc.mjs http://127.0.0.1:8545` → chainId `2205` |
| 6 | App smoke | `node scripts/devnet-erc20.mjs http://127.0.0.1:8545` |
| 7 | Soak | Keep process/containers up **24–48h**; restart once; re-run smoke |

### Staging ports & env

| Item | Default |
| :--- | :------ |
| JSON-RPC | `http://0.0.0.0:8545` → clients use `http://127.0.0.1:8545` |
| Chain ID | `2205` (`0x89d`) |
| Genesis | shared `genesis.json` (repo sample or `dew init --out …`) |
| Staking `0x102` | **off** (`--staking` only if intentional) |
| Native DewTx / precompiles | **on** (code defaults) |
| P2P encrypt | **on** |
| Dev faucet key | Anvil #0 only on **private** nets — see [Devnet](./devnet.md) |

Multi-process layout (encrypted P2P mesh, independent auto-mine nodes):

```bash
docker compose -f deploy/docker-compose.soak.yml --profile multi up --build
# RPC: :8545 :8546 :8547   P2P: :30303 :30304 :30305
```

## B. Public launch (after soak)

### Path B — controlled RPC on **one host** (fastest)

Full copy-paste runbook: [deploy/node/public-rpc-single-host.md](../../deploy/node/public-rpc-single-host.md).

**Compose (recommended packaging):**

```bash
docker compose -f deploy/docker-compose.soak.yml down
cp deploy/public.env.example deploy/public.env   # once
docker compose -f deploy/docker-compose.yml --env-file deploy/public.env up --build
# RPC via proxy: http://127.0.0.1  (TLS optional — see deploy/README.md)
node scripts/smoke-rpc.mjs http://127.0.0.1
```

| # | Step | Notes |
| -: | :--- | :---- |
| 1 | Dew not public | Compose: internal network only · systemd: `127.0.0.1:8545` only |
| 2 | TLS proxy | **Compose edge:** DNS-01 CF (`CERTBOT_AUTH=dns-cloudflare` + `CLOUDFLARE_API_TOKEN`) · or HTTP-01 webroot · or Origin PEMs + `CERTBOT_DISABLE=1` · see `deploy/README.md` · host nginx: `setup-certbot.sh` |
| 3 | Firewall | Allow 22/80/443 only — **not** 8545 / 8081 / 8082 |
| 4 | Feature flags | No `--staking`; Anvil keys **not** on public pages |
| 5 | Publish | HTTPS RPC + chain ID `2205` (bootnodes n/a for path B) |
| 6 | Smoke | `node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com` (or `http://127.0.0.1` pre-TLS) |
| 7 | Emergency | Stop proxy first (`docker compose stop proxy` / nginx); node can stay private |

### Path A — multi-host public (later)

**Prerequisite:** [D3a multi-process BFT](../scale/d3-scale.md#d3a--multi-process-dew-bft) implemented. Full operator spec: [D3d Path A](../scale/d3-scale.md#d3d--path-a-multi-host-public).

| # | Step | Notes |
| -: | :--- | :---- |
| 1 | New keys | Validators, bootnodes, faucet — **never** Anvil / staging keys |
| 2 | Freeze genesis | Same `chainId` / alloc / `initialValidators` on every host |
| 3 | Topology | ≥ 3 validators + optional non-validator RPC |
| 4 | Feature flags | Staking **off** unless operators agree; native/precompiles **on** |
| 5 | Publish | RPC URL, chain ID `2205`, bootnode list, faucet rate rules, explorer base or `(none)` |
| 6 | Faucet | Optional `dewfaucet` ([faucet.md](../product/faucet.md)): allowlist or captcha; rate limits; never Anvil keys |
| 7 | Monitor | RPC 4xx/5xx, mempool rejects, peer count; ready to stop RPC only |

### Public publish template (path B) — live

```text
Network:     Dew public-testnet-v1
Chain ID:    2205
RPC:         https://rpc-dew.fadosoft.com
Symbol:      DEW
Explorer:    https://explorer-dew.fadosoft.com
Faucet:      https://faucet-dew.fadosoft.com     # captcha · 1 DEW/address/24h · 10/IP/hour
Guestbook:   https://guestbook-dew.fadosoft.com
Guestbook:   0x83bB4E539BE46503481E66094b01b854990BF84a   # contract; SPA PUBLIC_GUESTBOOK
Bootnodes:   n/a (single-host controlled RPC)
```

Browser-only check: [Try public testnet](./try-public.md) (faucet → MetaMask → Guestbook → explorer).  
Block explorer URL conventions and MetaMask base URL: [Block explorer (web)](../product/block-explorer.md).  
Guestbook SPA + packaging: [Guestbook demo](../product/guestbook.md) · [`deploy/guestbook/`](../../deploy/guestbook/).  
Docker packaging: [`deploy/explorer/`](../../deploy/explorer/) (standalone) or combined [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml).  
Production faucet service: [Production faucet](../product/faucet.md) (`go build -o bin/dewfaucet ./cmd/dewfaucet`).

## C. Emergency stop

1. Stop **public RPC** (and faucet) first — `docker compose -f deploy/docker-compose.yml stop proxy` / nginx / `systemctl stop dewfaucet` / systemd stop RPC proxy
2. Disable native / staking if module bug (validators may stay up)  
3. Re-genesis only if wire freeze intentionally broken — coordinate publicly  

## Related

- [Public testnet freeze](./public-testnet.md)  
- [Try public testnet](./try-public.md)  
- [Block explorer (web)](../product/block-explorer.md)  
- [Guestbook demo](../product/guestbook.md)  
- [Private multi-host](./private-testnet.md)  
- [deploy/README](../../deploy/README.md)  
