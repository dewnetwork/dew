---
title: History indexer
description: Optional SQLite sidecar for explorer full history and volume charts (P1e).
category: product
order: 53
status: stable
---

# History indexer (`dewindex`)

Sidecar that indexes Dew **JSON-RPC** history into **SQLite** for explorer address history and volume charts. **Not** part of consensus or the wire freeze.

| Item | Value |
| :--- | :--- |
| Binary | `cmd/dewindex` · package `indexer/` |
| Default listen | `127.0.0.1:8550` |
| Store | SQLite (`-db` / `INDEXER_DB`) |
| Upstream | HTTP JSON-RPC (`-rpc` / `INDEXER_RPC_URL`) |

## Run (local)

```bash
# terminal 1
go run ./cmd/dew devnet --http.port 8545

# terminal 2
go run ./cmd/dewindex -rpc http://127.0.0.1:8545 -db /tmp/dew-indexer.db -http.addr 127.0.0.1:8550

# terminal 3 — explorer with indexer
PUBLIC_INDEXER_URL=http://127.0.0.1:8550 pnpm --dir explorer dev
```

## HTTP API

| Path | Description |
| :--- | :--- |
| `GET /health` | Liveness |
| `GET /v1/status` | `tip`, `indexed`, `lag`, chain id |
| `GET /v1/address/{addr}/txs?limit&offset` | Txs where address is from/to |
| `GET /v1/address/{addr}/transfers?limit&offset` | ERC-20 `Transfer` logs |
| `GET /v1/stats/volume?from&to` | Daily tx counts (UTC) over block range |

CORS: open `GET` / `OPTIONS` for browser explorer.

## Explorer

Set **`PUBLIC_INDEXER_URL`** at build/dev time (no trailing slash). When unset, explorer stays on the no-indexer MVP (RPC poll chart + balance only).

## Ingest

Polls `eth_blockNumber`, walks blocks with `eth_getBlockByNumber(full)` + `eth_getTransactionReceipt`. Checkpoint: `meta.last_indexed_block`. ERC-20 transfers decoded from topic0 `Transfer(address,address,uint256)`.

## Docker

```bash
docker build -f deploy/indexer/Dockerfile -t dew-indexer:local .
docker run --rm -p 8550:8550 \
  -e INDEXER_RPC_URL=http://host.docker.internal:8545 \
  dew-indexer:local
```

Compose snippet (optional service beside `dew-node`):

```yaml
indexer:
  build:
    context: ..
    dockerfile: deploy/indexer/Dockerfile
  command: ["-rpc", "http://dew-node:8545", "-db", "/data/indexer.db", "-http.addr", "0.0.0.0:8550"]
  volumes:
    - indexer-data:/data
  expose:
    - "8550"
```

Point explorer build arg / env `PUBLIC_INDEXER_URL` at the public or edge URL for the indexer (or same-origin proxy).

## Related

- [Product upgrades — P1e](./upgrades.md)
- [Block explorer](./block-explorer.md)
- [JSON-RPC](../api/json-rpc.md)
