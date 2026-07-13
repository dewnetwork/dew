---
title: Dew RPC Extensions
description: Phase B dew_* namespace for native txs and engine stats.
category: api
order: 20
status: stable
---

# Dew RPC Extensions

> Optional Dew methods. Do not block Ethereum tooling on these. Implemented alongside `eth_*` on the same HTTP server.

Namespace: `dew_*`.

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

## Other methods

Add methods additively under `dew_*`. Breaking changes require a new method name or explicit API version field before mainnet freeze. Validator-set RPC (if added) should reflect the live BFT set vs staking `ActiveSet` explicitly once D3c lands.

## Versioning

Prefer additive fields on existing responses (e.g. `dew_getExecutionStats`) over renames.
