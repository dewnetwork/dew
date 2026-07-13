---
title: Blocks
description: Block header and body structure for Dew (public-testnet-v1).
category: protocol
order: 40
status: stable
---

# Blocks

Header RLP field order and hash are consensus-critical under **`public-testnet-v1`**.

## Header

Go type: `core/types.Header`.

```go
type Header struct {
    ParentHash  [32]byte
    StateRoot   [32]byte
    TxRoot      [32]byte
    ReceiptRoot [32]byte
    Number      uint64
    Timestamp   uint64      // unix seconds
    GasLimit    uint64
    GasUsed     uint64
    BaseFee     *big.Int    // EIP-1559
    ExtraData   []byte      // max 32 bytes (enforced in validation paths)
    Proposer    [20]byte
}
```

Canonical RLP list order (do not reorder):

```text
[ParentHash, StateRoot, TxRoot, ReceiptRoot, Number, Timestamp,
 GasLimit, GasUsed, BaseFee, ExtraData, Proposer]
```

Consensus commit metadata (quorum certificates) may live **alongside** the block rather than inside `ExtraData`.

### Field rules

| Field | Rule |
| :--- | :--- |
| `ParentHash` | Hash of previous header; genesis uses zero hash |
| `Number` | Parent + 1 |
| `Timestamp` | Strictly greater than parent (nodes may advance by 1s if wall clock stalls) |
| `GasUsed` | ≤ `GasLimit` |
| `BaseFee` | Present; genesis base fee **1 gwei** on public-testnet-v1 |
| `StateRoot` | SMT root after executing included work (see [State](./state.md)) |
| `TxRoot` | Commitment over the EVM body list (see below) |
| `ReceiptRoot` | Commitment over receipts (may be empty-root when unused) |
| `Proposer` | Block proposer / fee sink address |

### Header hash

`blockHash = Keccak-256(RLP(header fields in canonical order))`.

## Body

```go
type Block struct {
    Header       *Header
    Transactions []*Transaction // EVM txs only under public-testnet-v1
}
```

- Phase A/C freeze: body is **EVM transactions only**
- `DewTx` inclusion is **receipt / tx-index backed**; pure-native auto-mine seals may have an empty body (see [Transactions](./transactions.md))
- A future tagged-union body (mixed EVM + DewTx) would be a deliberate hardfork, not a silent change

## Tree commitments

| Root | Construction (current) |
| :--- | :--- |
| `TxRoot` | `Keccak-256(RLP([txHash0, txHash1, …]))` over EVM body hashes; empty body → `EmptyTxRoot` |
| `ReceiptRoot` | Same family as empty list root when not fully trie-populated |
| `StateRoot` | Sparse Merkle Tree over account/storage/code leaves ([State](./state.md)) |

DewTx multi-pack seals that leave the body empty still set `TxRoot` to a hash-list commitment over included native hashes so the header is not all-zero.

## Block time

| Item | public-testnet-v1 practice |
| :--- | :--- |
| BFT multi-process pace | Default **1s** min interval between rounds (`MinBlockInterval`) |
| Path B auto-mine | Seals when ready pending exists (no fixed wall clock) |
| Empty blocks | Allowed so consensus can advance height |

## Genesis

Height 0 is defined by [Genesis](../economics/genesis.md). No transactions in the genesis body; state comes from `alloc`.
