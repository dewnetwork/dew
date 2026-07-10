---
title: Launch checklist
description: One-page private soak and public-testnet-v1 operator checklist.
category: development
order: 55
status: draft
---

# Launch checklist

Freeze tag: **`public-testnet-v1`** · chain ID **`2026`**.  
Packaging: [deploy/](../../deploy/) · freeze table: [Public testnet](./public-testnet.md) · private ops: [Private testnet](./private-testnet.md).

## A. Private staging (do this first)

| # | Step | Command / value |
| -: | :--- | :-------------- |
| 1 | Build binary | `go build -o bin/dew ./cmd/dew` |
| 2 | Chaos smoke | `go test ./devnet/ -run Chaos -count=1` |
| 3 | RPC abuse bar | `go test ./tests/security/ -run C6 -count=1` |
| 4 | Start private stack | **Compose:** `docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build` **or** `./bin/dew devnet --http.addr 0.0.0.0 --http.port 8545` |
| 5 | RPC smoke | `node scripts/smoke-rpc.mjs http://127.0.0.1:8545` → chainId `2026` |
| 6 | App smoke | `node scripts/devnet-erc20.mjs http://127.0.0.1:8545` |
| 7 | Soak | Keep process/containers up **24–48h**; restart once; re-run smoke |

### Staging ports & env

| Item | Default |
| :--- | :------ |
| JSON-RPC | `http://0.0.0.0:8545` → clients use `http://127.0.0.1:8545` |
| Chain ID | `2026` (`0x7ea`) |
| Genesis | shared `genesis.json` (repo sample or `dew init --out …`) |
| Staking `0x102` | **off** (`--staking` only if intentional) |
| Native DewTx / precompiles | **on** (code defaults) |
| P2P encrypt | **on** |
| Dev faucet key | Anvil #0 only on **private** nets — see [Devnet](./devnet.md) |

Multi-process layout (encrypted P2P mesh, independent auto-mine nodes):

```bash
docker compose -f deploy/docker-compose.yml --profile multi up --build
# RPC: :8545 :8546 :8547   P2P: :30303 :30304 :30305
```

## B. Public launch (after soak)

| # | Step | Notes |
| -: | :--- | :---- |
| 1 | New keys | Validators, bootnodes, faucet — **never** Anvil / staging keys |
| 2 | Freeze genesis | Same `chainId` / alloc / `initialValidators` on every host |
| 3 | Topology | ≥ 3 validators + optional non-validator RPC |
| 4 | Feature flags | Staking **off** unless operators agree; native/precompiles **on** |
| 5 | Publish | RPC URL, chain ID `2026`, bootnode list, faucet rate rules |
| 6 | Faucet | Rate limit per IP/address; small amounts; captcha recommended |
| 7 | Monitor | RPC 4xx/5xx, mempool rejects, peer count; ready to stop RPC only |

### Public publish template

```text
Network:     Dew public-testnet-v1
Chain ID:    2026
RPC:         https://rpc.example.com
Bootnodes:   host0:30303, host1:30304   # replace at launch
Faucet:      <URL or "request via …">   # policy: N DEW / address / day
Symbol:      DEW
```

## C. Emergency stop

1. Stop **public RPC** (and faucet) first — SIGTERM / `docker compose stop` / systemd stop  
2. Disable native / staking if module bug (validators may stay up)  
3. Re-genesis only if wire freeze intentionally broken — coordinate publicly  

## Related

- [Public testnet freeze](./public-testnet.md)  
- [Private multi-host](./private-testnet.md)  
- [deploy/README](../../deploy/README.md)  
