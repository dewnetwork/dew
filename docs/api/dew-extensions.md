---
title: Dew RPC Extensions
description: Phase B dew_* namespace for native txs and engine stats.
category: api
order: 20
status: draft
---

# Dew RPC Extensions

> **Phase B.** Do not block Ethereum tooling on these methods.

Namespace: `dew_*`.

## `dew_sendRawTransaction`

Submit a signed `DewTx` (hex).

- **Params**: `[serializedHex]`  
- **Result**: 32-byte tx hash  

Reject if native path feature flag is off.

## `dew_getExecutionStats`

Operational metrics for Dew-PE and DB:

```json
{
  "current_tps": 0,
  "peak_tps": 0,
  "active_workers": 0,
  "state_db_read_lat_ns": 0,
  "state_db_write_lat_ns": 0,
  "conflict_rollback_rate": 0.0
}
```

## `dew_getValidators`

Active validator set: address, voting power, jailed flag, optional proposed-block counters.

## Versioning

Add methods additively. Breaking changes require a new method name or explicit API version field before mainnet freeze.
