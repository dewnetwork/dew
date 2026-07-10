---
title: Dew RPC Extensions
description: Phase B dew_* namespace for native txs and engine stats.
category: api
order: 20
status: draft
---

# Dew RPC Extensions

> **Phase B.** Do not block Ethereum tooling on these methods. Implemented alongside `eth_*` on the same HTTP server.

Namespace: `dew_*`.

## `dew_sendRawTransaction`

Submit a signed `DewTx` (hex envelope `0xdf || RLP(...)`).

- **Params**: `[serializedHex]`  
- **Result**: 32-byte tx hash  

Reject if native path feature flag is off (`Node.SetNativeEnabled(false)`). Dev mode auto-mines one block per accepted DewTx (same as `eth_sendRawTransaction`).

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

## `dew_getValidators`

Active validator set: address, voting power, jailed flag, optional proposed-block counters.

## Versioning

Add methods additively. Breaking changes require a new method name or explicit API version field before mainnet freeze.
