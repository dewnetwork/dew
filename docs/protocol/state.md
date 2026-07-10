---
title: State
description: Account model, flat storage, and state root commitment.
category: protocol
order: 50
status: draft
---

# State

## Account object

```go
type Account struct {
    Nonce    uint64
    Balance  *uint256.Int // wei
    CodeHash [32]byte     // empty code hash for EOAs
    // Storage is keyed separately, not embedded in the account blob
}
```

Ethereum’s `storageRoot` may be retained for compatibility hashes or replaced by a global SMT layout — implementation choice, but **post-state must be deterministic** across nodes.

## Flat database model

Ethereum’s bottleneck is often MPT on the hot path. Dew uses:

| DB           | Mapping                                |
| :----------- | :------------------------------------- |
| `state_db`   | `address → account`                    |
| `storage_db` | `(address, slot) → value`              |
| `code_db`    | `codeHash → bytecode` (optional split) |

Execution reads/writes these KV stores (plus memory cache). No trie walk per opcode.

```mermaid
flowchart LR
  subgraph Hot["Hot path: flat KV"]
    EX[EVM / Executor] --> Cache[Memory cache]
    Cache --> SDB[(state_db)]
    Cache --> STDB[(storage_db)]
    Cache --> CDB[(code_db)]
  end
  subgraph Commit["Commit: SMT"]
    Dirty[Dirty accounts + slots] --> SMT[Sparse Merkle Tree]
    SMT --> Root[header.StateRoot]
  end
  Cache --> Dirty
```

## State root commitment

After all transactions in a block are applied:

1. Collect dirty accounts and storage slots (and code blobs) from the flat snapshot
2. Build a **Sparse Merkle Tree** (SMT) over those leaves
3. Set `header.StateRoot` to the SMT root
4. Validators re-execute (or verify execution proof path) and **require equal `StateRoot`**

### SMT algorithm (Phase C3)

Implemented in `core/state/smt.go` (commit-time only; hot path stays flat KV):

| Item | Spec |
| :--- | :--- |
| Leaf key path | `path = Keccak256(flatKey)` where flatKey is `a‖addr`, `s‖addr‖slot`, or `c‖codeHash` |
| Leaf hash | `Keccak256(0x00 ‖ value)` |
| Internal hash | `Keccak256(0x01 ‖ left ‖ right)` |
| Empty subtrees | Precomputed `emptyHashes[h]`; empty state root = `emptyHashes[256]` |
| Bit order | MSB of `path[0]` is the root branch bit |

Same pre-state + same txs ⇒ identical root on independent nodes. PE and sequential paths must still agree (tested via existing PE equivalence suites).

### Migration from provisional flat root

Phase A–B used a sorted-leaf `Keccak256(RLP(digests))` provisional root. That commitment is **obsolete**. Dev / private nets that stored old roots should **wipe and re-genesis**. No mainnet historical migration is required before C6 freeze.

### Async SMT?

Background construction is allowed **only as an optimization inside one process**. The root that appears in the proposed header must match what validators recompute before precommit. Consensus never finalizes a block with an “unknown” or deferred root.

## Journaling and reverts

EVM calls need snapshot/revert (gas OOG, `REVERT`, failed subcalls). Implement via journal or copy-on-write on the cache layer, same semantics as Ethereum.

## Phase B: MVCC cache

Parallel execution adds an in-block multi-version cache:

- Read/write sets per tx index
- Commit order preserves **serializable equivalent** of sequential index order

See [Parallel execution](../execution/parallel-execution.md).

## Pruning and archive (later)

Phase A full nodes may keep all blocks. Pruning policies are out of scope until after first multi-validator devnet.
