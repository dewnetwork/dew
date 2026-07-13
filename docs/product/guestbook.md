---
title: Guestbook (demo)
description: Public on-chain Guestbook SPA — contract, MetaMask sign, explorer deep-links, deploy packaging.
category: product
order: 54
status: stable
---

# Guestbook (demo)

The **Guestbook** is the flagship public demo for **public-testnet-v1**: append-only on-chain messages, a small SPA (read + MetaMask sign), and explorer deep-links. It is **ops / product surface**, not part of the consensus wire freeze.

| Item | Status |
| :--- | :--- |
| Solidity | [examples/foundry](../../examples/foundry/src/Guestbook.sol) · [examples/hardhat](../../examples/hardhat/contracts/Guestbook.sol) |
| SPA | [examples/guestbook-web](../../examples/guestbook-web/) |
| Live URL (path B) | `https://guestbook-dew.fadosoft.com` |
| Canonical contract (path B) | `0x83bB4E539BE46503481E66094b01b854990BF84a` |
| Wire / genesis impact | **None** — ordinary EVM contract + static UI |
| Chain ID | `2205` |

Freeze and live URLs: [Public testnet freeze](../ops/public-testnet.md#live-network-path-b). Browser-only walkthrough: [Try public testnet](../ops/try-public.md). Deploy scripts: [Builder recipes](../ops/recipes.md#recipe-2--on-chain-guestbook-flagship-demo).

## Why it exists

| Audience | Need |
| :--- | :--- |
| First-time builders | “I used Dew” without bridges or staking |
| Operators | Shareable URL next to RPC / faucet / explorer |
| Wallet users | Faucet → MetaMask → sign → see tx on explorer |

## Public surface (path B)

| Field | Value |
| :--- | :--- |
| SPA | `https://guestbook-dew.fadosoft.com` |
| Contract | `0x83bB4E539BE46503481E66094b01b854990BF84a` |
| RPC (UI default) | `https://rpc-dew.fadosoft.com` |
| Explorer | `https://explorer-dew.fadosoft.com` |

Ops may redeploy a new Guestbook address; set `PUBLIC_GUESTBOOK` and rebuild the SPA. Prefer keeping the publish template and this page in sync with the live table in [public-testnet.md](../ops/public-testnet.md).

## User flow

1. Get DEW from the [faucet](https://faucet-dew.fadosoft.com) (captcha).  
2. Open the SPA → **Connect** (MetaMask adds chain **2205** if missing).  
3. **Sign** a message (max 280 bytes on-chain).  
4. Open the **tx** / **address** deep-link on the explorer.  
5. **Refresh** reads entries via `eth_call` (no wallet required for read-only).

## Explorer deep-links

SPA and docs use the explorer **base URL only** (no path suffix in MetaMask). Patterns:

| Kind | URL shape |
| :--- | :--- |
| Transaction | `{EXPLORER}/tx/{hash}` |
| Address / contract | `{EXPLORER}/address/{addr}` |

Env: `PUBLIC_EXPLORER_URL` (build-time). Helpers live in `examples/guestbook-web/src/rpc.ts`. Route details: [Block explorer](./block-explorer.md).

## Env (SPA, build-time Vite)

| Variable | Default (public path B) |
| :--- | :--- |
| `PUBLIC_RPC_URL` | `https://rpc-dew.fadosoft.com` |
| `PUBLIC_GUESTBOOK` | `0x83bB4E539BE46503481E66094b01b854990BF84a` |
| `PUBLIC_EXPLORER_URL` | `https://explorer-dew.fadosoft.com` |
| `PUBLIC_CHAIN_ID` | `2205` |

Root scripts: `pnpm guestbook:dev` · `guestbook:build`. Local: [examples/guestbook-web/README.md](../../examples/guestbook-web/README.md).

## Deploy packaging

| Artifact | Role |
| :--- | :--- |
| [`deploy/guestbook/`](../../deploy/guestbook/) | Standalone Dockerfile / compose / env example |
| [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) | Full path B stack + edge host `guestbook-dew.fadosoft.com` |

```bash
# Standalone
cp deploy/guestbook/guestbook.env.example deploy/guestbook/guestbook.env
docker compose -f deploy/guestbook/docker-compose.yml \
  --env-file deploy/guestbook/guestbook.env up --build -d
```

Full edge: DNS A/AAAA → host, set `PUBLIC_GUESTBOOK` in `deploy/.env`, compose up, expand LE cert SAN for the Guestbook host. See [deploy/README.md](../../deploy/README.md).

## Out of scope

- Consensus, staking, or native DewTx requirements  
- Indexer / full-text search of messages  
- Path A multi-host topology ([D3 scale](../scale/d3-scale.md))  
- Replacing the wire freeze table with demo contract constants  

## Related

- [Try public testnet (5 minutes)](../ops/try-public.md)
- [Builder recipes](../ops/recipes.md)
- [Block explorer](./block-explorer.md)
- [Production faucet](./faucet.md)
- [Launch checklist](../ops/launch-checklist.md)
- [Public testnet freeze](../ops/public-testnet.md)
