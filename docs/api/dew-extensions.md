---
title: Dew RPC Extensions
description: dew_* namespace for native txs, PE stats, and mempool telemetry.
category: api
order: 20
status: stable
---

# Dew RPC Extensions

> Optional Dew methods. Do not block Ethereum tooling on these. Implemented alongside `eth_*` on the same HTTP server.

Namespace: `dew_*`.

Same HTTP limits as [JSON-RPC](./json-rpc.md) (C6): **1 MiB** body, **100** batch items. Telemetry methods are **read-only** and do not change admission policy or fee floors.

## `dew_sendRawTransaction`

Submit a signed `DewTx` (hex envelope `0xdf || RLP(...)`).

- **Params**: `[serializedHex]`
- **Result**: 32-byte tx hash

Reject if native path feature flag is off (`Node.SetNativeEnabled(false)`).

With **auto-mine** (Path B / local default): admits via the unified mempool, queues future nonces, and packs ready continuous nonce chains into one seal (up to 64), same packing rules as `eth_sendRawTransaction`. Wire and fees: [Transactions](../protocol/transactions.md).

## `dew_getExecutionStats`

Operational metrics for Dew-PE and DB:

```json
{
  "current_tps": 0,
  "peak_tps": 0,
  "active_workers": 0,
  "state_db_read_lat_ns": 0,
  "state_db_write_lat_ns": 0,
  "conflict_rollback_rate": 0.0,
  "tx_count": 0,
  "rollbacks": 0,
  "speculative_ok": 0,
  "total_txs": 0,
  "total_rollbacks": 0
}
```

Extra fields are additive (non-breaking).

## `dew_getMempoolStats`

Read-only unified mempool + fee-floor telemetry for lab load (Track 4 / S3).

- **Params**: none (`[]`)
- **Result**: object (fields below)

```json
{
  "pending": 0,
  "senders": 0,
  "pending_evm": 0,
  "pending_dew": 0,
  "max_global": 4096,
  "max_per_sender": 16,
  "max_tx_bytes": 131072,
  "min_gas_price_wei": "1000000000",
  "min_tip_wei": "1",
  "min_dew_fee_wei": 2100000000000,
  "price_bump_percent": 10,
  "admits": 0,
  "replaces": 0,
  "evictions": 0,
  "rejects": {
    "total": 0,
    "pool_full": 0,
    "sender_limit": 0,
    "underpriced": 0,
    "replace_underpriced": 0,
    "already_known": 0,
    "tx_too_large": 0,
    "invalid": 0,
    "wrong_chain": 0,
    "rbf_disabled": 0
  },
  "top_senders": [
    { "address": "0x…", "pending": 1 }
  ]
}
```

| Field | Meaning |
| :--- | :--- |
| `pending` / `senders` | Live pool size and unique senders |
| `pending_evm` / `pending_dew` | Kind split of current pending |
| `max_*` / `min_*` / `price_bump_percent` | **Configured** floors and limits (not market quotes) |
| `admits` / `replaces` / `evictions` | Lifetime counters since process start |
| `rejects.*` | Lifetime rejects by admission reason |
| `top_senders` | Up to **16** addresses with the most pending txs (desc) |

Notes:

- Fee floor values are **decimal strings** (or integers for Dew fee) so lab scripts can compare without float rounding.
- Counters are process-lifetime; they do **not** reset when txs are mined/removed.
- Does **not** implement full `txpool_content` / `txpool_inspect` (Ethereum optional APIs).
- Does **not** change mempool admission or fee floors.

Example:

```bash
curl -s -X POST http://127.0.0.1:8545 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"dew_getMempoolStats","params":[]}'
```

## Other methods

Add methods additively under `dew_*`. Breaking changes require a new method name or explicit API version field before mainnet freeze. Validator-set RPC (if added) should reflect the live BFT set vs staking `ActiveSet` explicitly once D3c lands.

## Versioning

Prefer additive fields on existing responses (e.g. `dew_getExecutionStats`, `dew_getMempoolStats`) over renames.
