# Dew Explorer

Read-only block explorer SPA for Dew (`public-testnet-v1`, chain ID **2205**). Consumes JSON-RPC only — no indexer, no keys.

Spec: [`docs/product/block-explorer.md`](../docs/product/block-explorer.md) · Phase **D1**: [`docs/build/phases.md`](../docs/build/phases.md).

## Stack

React · Vite · Tailwind · TanStack Router · TanStack Query · Radix · nuqs · Zustand

## Setup

```bash
# from monorepo root
pnpm --dir explorer install
cp explorer/.env.example explorer/.env   # optional

# local node (separate terminal)
go run ./cmd/dew devnet --http.port 8545

# explorer
pnpm explorer:dev
# → http://localhost:4321
```

### Environment

| Variable | Default | Role |
| :--- | :--- | :--- |
| `PUBLIC_RPC_URL` | `http://127.0.0.1:8545` | Browser JSON-RPC endpoint |
| `PUBLIC_CHAIN_ID` | `2205` | Must match network |
| `PUBLIC_EXPLORER_BASE` | _(empty)_ | Canonical origin for share links |

Vite is configured with `envPrefix: "PUBLIC_"`.

## Scripts

| Command | Description |
| :--- | :--- |
| `pnpm explorer:dev` | Dev server (port 4321) |
| `pnpm explorer:build` | Production build → `explorer/dist` |
| `pnpm explorer:preview` | Preview production build |

## Routes

| Path | Page |
| :--- | :--- |
| `/` | Home — stats, latest blocks & txs |
| `/block/$id` | Block by number or hash |
| `/tx/$hash` | Transaction + receipt / logs / **token transfers** |
| `/address/$addr` | Balance, nonce, code / **ERC-20 metadata** when applicable |

**Product-v1 / v1.1:** recent search chips; method labels; ERC-20 probe; Transfer/Approval decode; optional `PUBLIC_KNOWN_TOKENS` balance tab. Spec: [docs/product/upgrades.md](../docs/product/upgrades.md).

## Home network overview

Etherscan-style strip + **Transaction History in 14 days** (Recharts):

| Field | Source |
| :--- | :--- |
| DEW Price / Market Cap | Optional env (`PUBLIC_DEW_PRICE_USD`, …) — no on-chain oracle |
| Transactions + TPS | Sampled recent blocks + estimated 14d volume |
| Gas price | `eth_gasPrice` |
| Finalized / Safe block | Head (Dew-BFT committed ≈ final) |
| 14-day chart | Daily estimates from sparse `eth_getBlockByNumber` samples |

## Theme

Header control cycles **Light → Dark → System** (follows OS). Preference is stored under `localStorage` key `dew-explorer-ui` (`theme` field). Default: System.

## Deploy (Docker)

Packaging lives under [`deploy/explorer/`](../deploy/explorer/) (nginx SPA, not under this package’s build):

```bash
# from monorepo root
cp deploy/explorer/explorer.env.example deploy/explorer/explorer.env
docker compose -f deploy/explorer/docker-compose.yml --env-file deploy/explorer/explorer.env up --build -d
# → http://localhost:8082
```

Combined local stack (node + faucet + explorer): `docker compose -f deploy/docker-compose.yml up --build -d`.  
See [deploy/README.md](../deploy/README.md) and [block-explorer.md](../docs/product/block-explorer.md).

## Security

- Read-only UI (no `eth_sendRawTransaction`)
- Use a **proxied** public RPC in production (same abuse bar as path B)
- No secrets in frontend env — only public RPC URL and chain ID
