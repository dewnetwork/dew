---
title: Block explorer (web)
description: Public block explorer — MVP shipped, routes, RPC env, deploy, and publish template.
category: product
order: 52
status: stable
---

# Block explorer (web)

The **block explorer** is the public, read-only web UI for browsing Dew chain data. It is **ops / product surface**, not part of the consensus wire freeze.

| Item | Status |
| :--- | :--- |
| In-repo app | **MVP shipped** — `explorer/` (React + Vite + Tailwind) |
| Live URL (path B) | `https://explorer-dew.fadosoft.com` |
| Wire / genesis impact | **None** — JSON-RPC client only |
| Phase | **D1** done — [Phases](../build/phases.md#d1--block-explorer-mvp) |

Freeze tag and RPC limits: [Public testnet freeze](../ops/public-testnet.md). Operator checklist: [Launch checklist](../ops/launch-checklist.md). App README: [`explorer/README.md`](../../explorer/README.md).

## Why it exists

| Audience | Need |
| :--- | :--- |
| Wallet / dApp users | Paste a tx hash or address without CLI |
| Operators | Canonical link when publishing the network |
| Tooling | Deep-link from MetaMask, faucet, Guestbook SPA, docs |

The explorer **must not** hold validator keys, faucet keys, or admin signing material.

## MVP (shipped)

### Routes

| Route | Purpose |
| :--- | :--- |
| `/` | Stats + latest blocks + latest txs (poll while tab visible) |
| `/block/{n\|hash}` | Header fields + tx list |
| `/tx/{hash}` | Status, fees, gas, input, logs, **token transfers** (product-v1) |
| `/address/{addr}` | Balance, nonce, code / EOA vs contract; **ERC-20 metadata** when `eth_call` succeeds (product-v1) |

Search resolves address / tx hash / block number (and block hash when applicable). **Recent searches** chips (localStorage). Failed, pending, and not-found states are distinct.

### Product-v1 (no indexer)

| Feature | Behavior |
| :--- | :--- |
| ERC-20 probe | `name` / `symbol` / `decimals` / `totalSupply` via `eth_call`; Token badge + tab |
| Method labels | Common selectors (`transfer`, `approve`, …) on tx overview |
| Token transfers tab | Decode ERC-20 `Transfer` / `Approval` logs from receipt |
| Recent search | Up to 8 queries in `localStorage` (`dew-explorer-ui`) |
| Known-token balances | `PUBLIC_KNOWN_TOKENS` (`SYMBOL:0xaddr` list) → `balanceOf` tab on address page |

Env: `PUBLIC_KNOWN_TOKENS` optional. Upgrade backlog: [upgrades.md](./upgrades.md).

### Deep-links from demos

Consumers should use the explorer **base URL** only (MetaMask “Block explorer URL” = base, no path). Append routes in apps:

| Kind | Pattern |
| :--- | :--- |
| Transaction | `{BASE}/tx/{hash}` |
| Address | `{BASE}/address/{addr}` |
| Block | `{BASE}/block/{n\|hash}` |

**Shipped consumer:** [Guestbook SPA](./guestbook.md) (`PUBLIC_EXPLORER_URL`) links after sign and for author/contract addresses. Browser walkthrough: [Try public testnet](../ops/try-public.md).

### Env (build-time)

| Variable | Example |
| :--- | :--- |
| `PUBLIC_RPC_URL` | `https://rpc-dew.fadosoft.com` or `http://127.0.0.1:8545` |
| `PUBLIC_CHAIN_ID` | `2205` |
| `PUBLIC_EXPLORER_BASE` | optional public base URL |

Root scripts: `pnpm explorer:dev` · `explorer:build` · `explorer:preview`.

### Local DX

```bash
# Terminal 1 — chain
go build -o bin/dew ./cmd/dew && ./bin/dew devnet --http.port 8545

# Terminal 2 — explorer
pnpm --dir explorer install
PUBLIC_RPC_URL=http://127.0.0.1:8545 PUBLIC_CHAIN_ID=2205 pnpm explorer:dev
# → http://localhost:4321 (see explorer package scripts)
```

## Design bar (summary)

- **Glanceable home** — height / recent activity without full indexer
- **Every hash actionable** — truncate in tables; full value + copy on detail
- **Honest empties** — pending / not found / RPC error, never a blank page
- **Finality** — Dew-BFT committed ≈ final; prefer **Final** over Ethereum-style confirmation theater
- **Theme** — light/dark; explorer-local tokens (not required to match marketing `web/`)

Deep UI field lists and Etherscan-class IA notes that drove the MVP live in git history of this page and in `explorer/src/` — this doc tracks **operator + shipped surface**.

## Deploy packaging

| Artifact | Role |
| :--- | :--- |
| [`deploy/explorer/`](../../deploy/explorer/) | Dockerfile, compose, nginx SPA `try_files`, env example |
| [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) | Combined stack: node + faucet + explorer |

```bash
cp deploy/explorer/explorer.env.example deploy/explorer/explorer.env
# set PUBLIC_RPC_URL / PUBLIC_EXPLORER_BASE
docker compose -f deploy/explorer/docker-compose.yml --env-file deploy/explorer/explorer.env up --build -d
```

`PUBLIC_*` values are **build-time** (Vite). Rebuild after changing RPC or base URL. Browser must reach `PUBLIC_RPC_URL` (host or public HTTPS — not Docker-internal hostnames).

## Security

| Rule | Why |
| :--- | :--- |
| Read-only | No send-tx from explorer UI |
| No secrets in frontend env | Only public RPC URL and chain ID |
| HTTPS in production | Match wallet expectations |
| Independent of validators | Explorer outage must not stop the chain |

## Operator publish snippet

1. Deploy UI with `PUBLIC_RPC_URL` → public HTTPS RPC  
2. Confirm `eth_chainId` is `0x89d` (2205)  
3. Smoke `/`, `/block/{n}`, `/tx/{hash}`, `/address/{addr}`  
4. MetaMask “Block explorer URL” = **base** only  
5. Publish text: `Explorer: https://explorer-dew.fadosoft.com` (or your host)

## Related

- [Launch checklist](../ops/launch-checklist.md)
- [Public testnet freeze](../ops/public-testnet.md)
- [Try public testnet](../ops/try-public.md)
- [JSON-RPC](../api/json-rpc.md)
- [Devnet](../ops/devnet.md)
- [deploy packaging](../../deploy/README.md)
- [Faucet](./faucet.md)
- [Guestbook demo](./guestbook.md)
