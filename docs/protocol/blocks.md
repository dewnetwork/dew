---
title: Blocks
description: Block header and body structure for Dew.
category: protocol
order: 40
status: draft
---

# Blocks

## Header

```go
type BlockHeader struct {
    ParentHash   [32]byte
    StateRoot    [32]byte
    TxRoot       [32]byte
    ReceiptRoot  [32]byte
    BlockNumber  uint64
    Timestamp    uint64      // unix seconds
    GasLimit     uint64
    GasUsed      uint64
    BaseFee      *big.Int    // EIP-1559; required in Phase A
    ExtraData    []byte      // max 32 bytes (*tentative*)
    Proposer     [20]byte
    // Consensus commit metadata may be stored alongside the block
    // (quorum certificate) rather than inside ExtraData long-term.
}
```

### Field rules

| Field         | Rule                                                                              |
| :------------ | :-------------------------------------------------------------------------------- |
| `ParentHash`  | Hash of previous header; genesis uses zero hash                                   |
| `BlockNumber` | Parent + 1                                                                        |
| `Timestamp`   | Strictly greater than parent; within +5s of local clock for prevote (_tentative_) |
| `GasUsed`     | ≤ `GasLimit`                                                                      |
| `BaseFee`     | Updated each block per EIP-1559 elasticity rules                                  |
| `StateRoot`   | Root after executing all txs in this block                                        |
| `TxRoot`      | Root of transaction list                                                          |
| `ReceiptRoot` | Root of receipt list                                                              |

### Header hash

`blockHash = Keccak-256(canonical_header_encoding)`. Encoding must be fixed (RLP recommended for ETH familiarity) before testnet freeze.

## Body

```go
type Block struct {
    Header       *BlockHeader
    Transactions []Transaction // Phase A: EVM txs only
}
```

Phase B may allow mixed EVM + `DewTx` lists with a tagged union encoding.

## Merkle / tree commitments

| Root          | Recommended construction (Phase A)                                                                          |
| :------------ | :---------------------------------------------------------------------------------------------------------- |
| `TxRoot`      | Merkle root over canonical tx encodings (Ethereum-style tx trie or binary merkle — **pick one and freeze**) |
| `ReceiptRoot` | Same scheme over receipts                                                                                   |
| `StateRoot`   | Sparse Merkle Tree over account (and storage) commitments                                                   |

Until freeze, implementers should isolate hashing behind an interface so the tree type can be swapped without rewriting execution.

## Block time target

- **Target**: 1 second (_tentative_)
- Empty blocks: allowed if required by consensus timeouts to advance height
- Proposer builds from mempool under `GasLimit`

## Genesis

Height 0 is defined by [Genesis](../economics/genesis.md). No transactions in the genesis body; state comes from `alloc`.
