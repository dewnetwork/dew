---
title: JSON-RPC (Ethereum)
description: eth_*, net_*, and web3_* methods for wallet and tooling compatibility.
category: api
order: 10
status: draft
---

# JSON-RPC (Ethereum)

## Endpoints

| Protocol | Default port |
| :--- | :--- |
| HTTP | `8545` |
| WebSocket | `8546` |

Content-Type: `application/json`. JSON-RPC 2.0.

### Public testnet resource limits (C6)

| Limit | Value |
| :--- | :--- |
| Max HTTP body | 1 MiB (`rpc.MaxRequestBodyBytes`) |
| Max batch items | 100 (`rpc.MaxBatchItems`) |

Oversized body/batch → JSON-RPC error `-32600`. Tx admission also enforces mempool fee floors and size caps (C1). See [Public testnet freeze](../development/public-testnet.md).

## Phase A — required methods

### Node identity

| Method | Notes |
| :--- | :--- |
| `web3_clientVersion` | e.g. `Dew/v0.1.0/go1.22` |
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
| `eth_getLogs` | Address/topic filters |
| `eth_newFilter` / `eth_getFilterChanges` | Optional Phase A stretch |
| `eth_subscribe` (WS) | `newHeads`, `logs` — strongly recommended |

## Block tags

Support: `latest`, `earliest`, `pending` (pending may equal latest in early versions if no pending state).

## Error shape

Standard JSON-RPC errors; use Ethereum-like error codes where tooling expects them (e.g. intrinsic gas too low).

## Out of scope for Phase A

- `debug_*` full traces (add when needed for Foundry traces)  
- `txpool_*` (optional)  
- Engine API (not a CL/EL split node)
