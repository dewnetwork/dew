# Dew Guestbook (web)

SPA for the on-chain [Guestbook](../foundry/src/Guestbook.sol) on **public-testnet-v1** (chain **2205**).

- **Read** messages via JSON-RPC (`eth_call`)
- **Sign** with MetaMask (adds chain 2205 if missing)
- **Burst ×2 (C1)** — two consecutive nonces to demo multi-tx packing
- Default contract: `0x83bB4E539BE46503481E66094b01b854990BF84a`

## Local

```bash
cd examples/guestbook-web
pnpm install
pnpm dev          # http://localhost:4323
```

From monorepo root: `pnpm guestbook:dev` · `pnpm guestbook:build`

## Env (build-time Vite)

| Variable | Default |
| :--- | :--- |
| `PUBLIC_RPC_URL` | `https://rpc-dew.fadosoft.com` |
| `PUBLIC_GUESTBOOK` | `0x83bB4E…F84a` |
| `PUBLIC_EXPLORER_URL` | `https://explorer-dew.fadosoft.com` |
| `PUBLIC_CHAIN_ID` | `2205` |

## Public host (Docker)

Standalone:

```bash
cp deploy/guestbook/guestbook.env.example deploy/guestbook/guestbook.env
docker compose -f deploy/guestbook/docker-compose.yml \
  --env-file deploy/guestbook/guestbook.env up --build -d
# http://127.0.0.1:8083
```

Full stack edge (`guestbook-dew.fadosoft.com`):

1. DNS A/AAAA → host  
2. `PUBLIC_GUESTBOOK=…` in `deploy/.env`  
3. `docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build -d`  
4. Re-issue/expand LE cert to include `guestbook-dew.fadosoft.com` (certbot SAN)

See [deploy/README.md](../../deploy/README.md).

## MetaMask

Network is auto-added on first connect/sign if missing:

| Field | Value |
| :--- | :--- |
| Chain ID | `2205` |
| RPC | `https://rpc-dew.fadosoft.com` |
| Symbol | DEW |
| Explorer | `https://explorer-dew.fadosoft.com` |

Get DEW: [faucet](https://faucet-dew.fadosoft.com).

Recipes: [docs/ops/recipes.md](../../docs/ops/recipes.md).
