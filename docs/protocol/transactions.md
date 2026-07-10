---
title: Transactions
description: EVM transaction formats (Phase A) and Dew-native transactions (Phase B).
category: protocol
order: 30
status: draft
---

# Transactions

## Phase A: EVM transactions (required)

Phase A supports Ethereum-compatible transactions so existing wallets work.

### EIP-1559 (type `0x02`) — primary

Canonical form:

```
0x02 || RLP([
  chainId,
  nonce,
  maxPriorityFeePerGas,
  maxFeePerGas,
  gasLimit,
  to,              // empty for contract creation
  value,
  data,
  accessList,      // EIP-2930 style list; may be empty
  yParity, r, s
])
```

### Legacy (type legacy / optional)

Support for type-0 legacy txs is **recommended** for older tooling. If implemented, enforce EIP-155 chain ID in the signature.

### Validation checklist

1. RLP decodes successfully
2. Signature recovers to a valid address
3. `chainId` matches network
4. Nonce matches sender account (or is next pending in mempool)
5. `maxFeePerGas >= baseFee` (when base fee is active)
6. Sender balance covers `value + gasLimit * maxFeePerGas` (conservative check)
7. Intrinsic gas ≤ `gasLimit`

### Transaction hash

`txHash = Keccak-256(signed_tx_bytes)` per Ethereum rules for the type.

## Phase B: Dew-native transactions (`DewTx`)

**Not required for first devnet.** Introduced after EVM path is stable.

Design goals: smaller payload, explicit access lists for lock-free scheduling, fixed micro-fee path.

```go
type DewTx struct {
    Version    uint32
    Nonce      uint64
    Sender     [20]byte
    Receiver   [20]byte
    Amount     *uint256.Int // DEW in wei
    Fee        uint64       // flat fee in wei (normative for Phase B)
    Payload    []byte       // native call data
    AccessList [][20]byte   // declared read/write accounts
    Signature  []byte       // 65-byte compact ECDSA
}
```

### Encoding (frozen Phase B)

- Versioned binary: `0xdf || RLP([version, chainId, nonce, sender, receiver, amount, fee, payload, accessList, yParity, r, s])`.
- RPC submit via `dew_sendRawTransaction` (hex of signed bytes).
- Signing hash = `Keccak-256(Keccak-256("DewTx:v1") || RLP(unsigned fields))` — domain ≠ EVM tx hash.

### Fee rule (frozen Phase B)

Flat fee in wei: `params.DefaultDewTxFeeWei = 2_100_000_000_000` (~10% of 21_000 gas × 1 gwei). Paid to block proposer. Not EVM gas.

## Receipts

Every included transaction produces a receipt (EVM path):

| Field               | Notes                     |
| :------------------ | :------------------------ |
| `status`            | `1` success / `0` failure |
| `gasUsed`           | Actual gas consumed       |
| `logs`              | EVM logs                  |
| `cumulativeGasUsed` | Within block              |
| `effectiveGasPrice` | Per EIP-1559 rules        |
| `contractAddress`   | Set on create             |

Receipt root is committed in the block header. Exact trie encoding: see [Blocks](./blocks.md).
