---
title: Node Internals
description: Internal subsystems, interfaces, and lifecycle of a Dewchain node.
category: architecture
order: 20
status: draft
---

# Node Internals

## Process lifecycle

```
1. Load config + genesis
2. Open databases (block, state, peer store)
3. Init genesis if empty chain
4. Start P2P host
5. Start consensus reactor (validator or observer)
6. Start RPC servers (HTTP 8545, WS 8546 by default)
7. Signal ready; handle SIGINT/SIGTERM gracefully
```

## Core interfaces (conceptual Go)

These are design-level interfaces, not frozen APIs:

```go
// Executor applies a block's transactions and returns results + new state root.
type Executor interface {
    Execute(block *types.Block, state State) (*types.BlockResult, error)
}

// State is the account/storage view used during execution.
type State interface {
    GetAccount(addr common.Address) (*types.Account, error)
    SetAccount(addr common.Address, acc *types.Account) error
    GetState(addr common.Address, key common.Hash) common.Hash
    SetState(addr common.Address, key, val common.Hash)
    Commit() (root common.Hash, err error)
    // ...
}

// ConsensusEngine drives height/round and talks to networking.
type ConsensusEngine interface {
    Start(ctx context.Context) error
    // ...
}
```

Phase A `Executor` is sequential EVM. Phase B swaps or wraps with Dew-PE while preserving `BlockResult` semantics.

## Mempool

Responsibilities:

- Admit txs with valid signature and chain ID
- Enforce nonce ordering per sender
- Reject grossly underpriced txs (base fee rules)
- Evict on replace (higher tip) or size limits
- Feed proposer and gossip

Phase A can use a simple in-memory pool. Do not over-design before multi-node testing.

## Databases

| Store      | Keys (illustrative) | Values                 |
| :--------- | :------------------ | :--------------------- |
| Block DB   | hash, height        | header, body, receipts |
| State DB   | address             | account RLP/binary     |
| Storage DB | address \|\| slot   | 32-byte value          |
| Peer store | node ID             | addresses, last seen   |
| Consensus  | height/round meta   | votes, valset          |

Exact codec is implementation-defined but must be versioned.

## Configuration surfaces

| Input                 | Examples                                            |
| :-------------------- | :-------------------------------------------------- |
| `genesis.json`        | chainId, alloc, initialValidators, consensus params |
| `config.toml` / flags | listen addrs, RPC, data dir, priv validator key     |
| Runtime               | log level, max peers                                |

See [Genesis](../economics/genesis.md).

## Observability (minimum)

- Structured logs for consensus rounds and RPC errors
- Counters: txs admitted, blocks committed, peer count
- Phase B: conflict/rollback rate for Dew-PE

Expose advanced stats later via `dew_getExecutionStats` ([Dew RPC extensions](../api/dew-extensions.md)).
