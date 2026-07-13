---
title: Transactions
description: EVM transaction formats and Dew-native transactions (public-testnet-v1).
category: protocol
order: 30
status: stable
---

# Transactions

Wire surfaces below match freeze tag **`public-testnet-v1`** (`params/freeze.go`). Prefer config changes over wire churn.

## EVM transactions

Ethereum-compatible transactions so existing wallets and toolchains work (MetaMask, Foundry, Hardhat).

### EIP-1559 (type `0x02`) — primary

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

### Legacy (type 0)

Type-0 legacy txs are supported for older tooling. EIP-155 chain ID is enforced in the signature path.

### Validation checklist

1. RLP decodes successfully
2. Signature recovers to a valid address
3. `chainId` matches network (**2205** on public-testnet-v1)
4. Nonce: reject only if `<` account nonce; higher nonces may stay pending (gap queue)
5. Fee floors: mempool min gas price / tip (see [Gas and fees](../execution/gas-and-fees.md))
6. Intrinsic gas ≤ `gasLimit` at execution
7. Balance covers gas + value at execution (hard fail if not)

### Transaction hash

`txHash = Keccak-256(signed_tx_bytes)` per Ethereum rules for the type.

### Multi-tx packing (C1)

Under auto-mine / proposal selection, continuous per-sender nonce chains pack into one block (up to **64** txs) with a fee auction across ready senders. See [Phases — C1](../build/phases.md#c1--mempool-admission--fee-policy).

## Dew-native transactions (`DewTx`)

Optional high-performance path alongside EVM. Domain-separated wire; not an Ethereum typed-tx byte.

```go
type DewTx struct {
    Version    uint32
    ChainID    *big.Int
    Nonce      uint64
    Sender     [20]byte
    Receiver   [20]byte
    Amount     *uint256.Int // DEW in wei
    Fee        uint64       // flat fee in wei
    Payload    []byte       // native module data
    AccessList [][20]byte   // declared accounts (fail-closed)
    V, R, S    *big.Int     // ECDSA recovery (yParity, r, s)
}
```

### Encoding (frozen)

- Wire: `0xdf || RLP([version, chainId, nonce, sender, receiver, amount, fee, payload, accessList, yParity, r, s])`
- Prefix `0xdf` is outside Ethereum typed-tx range so accidental `eth_sendRawTransaction` fails closed
- RPC: `dew_sendRawTransaction` (hex of signed bytes)
- Signing hash: `Keccak-256(Keccak-256("DewTx:v1") || RLP(unsigned fields))` — ≠ EVM tx hash

### Fee rule (frozen)

| Parameter | Value |
| :--- | :--- |
| `DefaultDewTxFeeWei` / `MinDewTxFeeWei` | `2_100_000_000_000` (~10% of 21_000 gas × 1 gwei) |

Paid to the block proposer (fee sink). Not EVM gas. See [Gas and fees](../execution/gas-and-fees.md).

### Inclusion model (public-testnet-v1)

- Unified mempool with EVM (`mempool.Pool`); same gap-queue / multi-tx auto-mine packing rules
- **Block body** still carries EVM `Transaction` list only — no tagged-union body under this freeze
- Included DewTxs are applied by the native executor and indexed via **receipts / tx lookup** (body may be empty for pure-native seals)
- Full mixed body encoding remains a post-freeze / hardfork design item

## Receipts

Every included transaction produces a receipt:

| Field | EVM | DewTx |
| :--- | :--- | :--- |
| `status` | `1` / `0` | success path is `1` (fail-closed incomplete access list is not included) |
| `gasUsed` | actual EVM gas | `0` (flat fee path) |
| `logs` | EVM logs | none |
| `cumulativeGasUsed` | within block | `0` for native-only |
| `effectiveGasPrice` | EIP-1559 effective | `0` |
| `contractAddress` | set on create | n/a |
| `type` | EVM type | `0xdf` (`types.DewTxType`) |

Header `ReceiptRoot` commitment scheme: see [Blocks](./blocks.md).
