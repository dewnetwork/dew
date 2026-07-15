---
title: JSON-RPC (Ethereum)
description: eth_*, net_*, and web3_* methods for wallet and tooling compatibility.
category: api
order: 10
status: stable
---

# JSON-RPC (Ethereum)

## Endpoints

| Protocol | Default port | Status |
| :--- | :--- | :--- |
| HTTP | `8545` | **Implemented** (`rpc` package) |
| WebSocket | same as HTTP | **Implemented** — upgrade on the HTTP listener (`Upgrade: websocket`); no separate default port |

Content-Type: `application/json`. JSON-RPC 2.0 over **HTTP** and **WebSocket** (same bind address). Subscriptions require WebSocket.

### Public testnet RPC (live)

| Item | Value |
| :--- | :--- |
| URL | `https://rpc-dew.fadosoft.com` |
| Chain ID | `2205` (`eth_chainId` → `0x89d`) |
| Smoke | `node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com` |

See [Public testnet freeze](../ops/public-testnet.md#live-network-path-b) for explorer, faucet, and MetaMask fields.

### Public testnet resource limits (C6)

| Limit | Value |
| :--- | :--- |
| Max HTTP body | 1 MiB (`rpc.MaxRequestBodyBytes`) |
| Max batch items | 100 (`rpc.MaxBatchItems`) |

Oversized body/batch → JSON-RPC error `-32600`. Tx admission also enforces mempool fee floors and size caps (C1). See [Public testnet freeze](../ops/public-testnet.md).

## Phase A — required methods

### Node identity

| Method | Notes |
| :--- | :--- |
| `web3_clientVersion` | e.g. `Dew/v0.2.0/go1.25.x` (semver from `version.Version` / release ldflags; local builds often `Dew/vdev/…`) |
| `net_version` | Decimal network id string, e.g. `"2205"` |
| `eth_chainId` | Hex chain id, e.g. `"0x89d"` (2205) |
| `net_listening` | Boolean |
| `net_peerCount` | Hex peer count |

### Chain head and blocks

| Method | Notes |
| :--- | :--- |
| `eth_blockNumber` | Latest height |
| `eth_getBlockByNumber` | Full or hash-only txs |
| `eth_getBlockByHash` | Same |
| `eth_getBlockTransactionCountByNumber` | Optional but useful |

### State

| Method | Notes |
| :--- | :--- |
| `eth_getBalance` | Address + block tag |
| `eth_getTransactionCount` | Nonce |
| `eth_getCode` | Contract bytecode |
| `eth_getStorageAt` | Slot read |

### Transactions

| Method | Notes |
| :--- | :--- |
| `eth_sendRawTransaction` | RLP signed tx |
| `eth_call` | Eth_call simulation |
| `eth_estimateGas` | Gas estimate |
| `eth_getTransactionByHash` | Tx lookup |
| `eth_getTransactionReceipt` | Receipt + logs |

### Fees

| Method | Notes |
| :--- | :--- |
| `eth_gasPrice` | Legacy helper |
| `eth_maxPriorityFeePerGas` | Tip suggestion |
| `eth_feeHistory` | Recommended for EIP-1559 wallets |

### Logs

| Method | Notes |
| :--- | :--- |
| `eth_getLogs` | Address/topic filters; durable O(range) via secondary log index |
| `eth_newFilter` / `eth_getFilterChanges` / `eth_getFilterLogs` / `eth_uninstallFilter` | **Implemented** (HTTP poll); max **128** filters; **5m** idle TTL; cursor-by-height |
| `eth_newBlockFilter` | **Implemented** — changes are block hashes since last poll |
| `eth_newPendingTransactionFilter` | **Implemented** — changes always empty until mempool stream |
| `eth_subscribe` (WS) | **Implemented:** `newHeads`, `logs` (optional address/topics filter) |
| `eth_unsubscribe` (WS) | **Implemented** |

WebSocket: dial `ws://host:port/` (or `wss://` behind TLS). Notifications:

```json
{"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"0x…","result":{…}}}
```

Limits: max **16** subscriptions per connection, max **256** concurrent WS connections (`rpc.MaxWSSubscriptionsPerConn` / `MaxWSConnections`). HTTP `eth_subscribe` returns an error (use WS).

## Block tags

Support: `latest`, `earliest`, `pending` (pending may equal latest in early versions if no pending state).

## Error shape

Standard JSON-RPC errors; use Ethereum-like error codes where tooling expects them (e.g. intrinsic gas too low).

## Dew extensions (same HTTP server)

| Method | Notes |
| :--- | :--- |
| `dew_sendRawTransaction` | Native DewTx submit |
| `dew_getExecutionStats` | PE / operational metrics |
| `dew_getMempoolStats` | Mempool size, fee floors, admit/reject/evict counters (read-only) |

Full shapes: [Dew RPC extensions](./dew-extensions.md).

## Out of scope for Phase A

- `debug_*` full traces (add when needed for Foundry traces)  
- Full Ethereum `txpool_*` content/inspect (Dew exposes summary telemetry via `dew_getMempoolStats` instead)  
- Engine API (not a CL/EL split node)
