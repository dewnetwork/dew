---
title: Architecture Overview
description: High-level node architecture and data flow for Dew.
category: architecture
order: 10
status: draft
---

# Architecture Overview

Dew is developed as a **monorepo**: the **Go** process is the chain node; **Node.js** builds the **docs website** (and later tooling) and does **not** run consensus. See [Monorepo layout](../development/go-project-layout.md).

## Node at a glance

A full Dew node is a single Go process that:

1. Accepts transactions (RPC / P2P)
2. Holds them in a mempool
3. Participates in Dew-BFT (if validator) or follows commits (if full node)
4. Executes transactions against state
5. Persists blocks and state
6. Serves JSON-RPC to wallets and dapps

```mermaid
flowchart LR
  W[Wallets / tools] --> RPC[JSON-RPC]
  Peers[Peers] <--> P2P[P2P]
  RPC --> MP[Mempool]
  P2P --> MP
  MP --> C[Consensus Dew-BFT]
  C --> EX[Execution EVM]
  EX --> DB[(State + Block DB)]
  P2P <--> C
  RPC -.-> DB
```

## Component map

| Component        | Package (target)             | Responsibility                      |
| :--------------- | :--------------------------- | :---------------------------------- |
| CLI / node entry | `cmd/dew`, `cmd/dewcli` | Process bootstrap, wallet utilities |
| Types            | `core/types`                 | Block, header, tx, receipt          |
| State            | `core/state`                 | Accounts, storage, commit, roots    |
| VM               | `core/vm`                    | EVM wrapper + StateDB bridge        |
| Crypto           | `crypto`                     | Keys, sign, verify, hashes          |
| Database         | `db`                         | KV backend (LevelDB/Pebble/RocksDB) |
| Consensus        | `consensus`                  | Dew-BFT state machine               |
| P2P              | `p2p`                        | Peers, gossip, sync                 |
| RPC              | `rpc`                        | HTTP/WS JSON-RPC                    |
| Config           | `config`                     | Genesis + node config               |

## Critical data paths

### A. User transaction (Phase A)

```mermaid
sequenceDiagram
  participant Client
  participant RPC as JSON-RPC
  participant MP as Mempool
  participant P2P
  participant Prop as Proposer
  participant Vals as Validators
  participant DB as State/Block DB

  Client->>RPC: eth_sendRawTransaction
  RPC->>RPC: decode RLP + verify ECDSA + chainId
  RPC->>MP: admit (nonce / balance)
  MP->>P2P: gossip Inventory / Tx
  Prop->>MP: pull txs into block
  Prop->>Vals: Proposal (header + body)
  Vals->>Vals: execute sequentially
  Vals->>Vals: StateRoot + ReceiptRoot match
  Vals->>Vals: Prevote → Precommit → Commit
  Vals->>DB: persist block + apply state
```

### B. State commitment (normative)

Unlike a pure “async root after the fact” design, **validators must agree on `StateRoot` at commit time**:

1. Execute all txs in the block (Phase A: sequential; Phase B: parallel with equivalent result)
2. Compute `StateRoot` (and tx/receipt roots)
3. Header must contain those roots before votes that finalize the block

Background SMT construction is an **implementation optimization** only if it does not change the hash validators vote on. If root calculation is slow, optimize the tree implementation — do not skip root in the header for Phase A.

### C. Phase B extension (not on critical path for v0)

```mermaid
flowchart TD
  MP[Mempool] --> Hints[Optional access-list hints]
  Hints --> PE[Dew-PE workers + MVCC]
  PE --> Val{Validate read sets}
  Val -->|conflict| Re[Re-execute Ti + dependents]
  Re --> Val
  Val -->|ok| Root[Same StateRoot as sequential]
```

Parallel execution is a **performance layer**. Consensus still verifies the same post-state root.

## Deployment roles

| Role              |   RPC    |   P2P   | Propose blocks |         Vote         |
| :---------------- | :------: | :-----: | :------------: | :------------------: |
| Validator         | optional |   yes   | when selected  |         yes          |
| Full node         |   yes    |   yes   |       no       | no (follows commits) |
| RPC-only (future) |   yes    | limited |       no       |          no          |

```mermaid
flowchart TB
  subgraph Validators
    V1[Validator A]
    V2[Validator B]
    V3[Validator C]
  end
  FN[Full node] --> V1
  FN --> V2
  FN --> V3
  RPC[RPC-only future] -.-> FN
  Apps[Wallets / dapps] --> RPC
  Apps --> FN
```

## Design boundaries

- **Consensus does not embed EVM logic** — it calls an `Executor` interface.
- **RPC does not talk to DB bypassing state layer** — queries go through chain/state APIs.
- **P2P does not interpret contract ABI** — only tx bytes, blocks, and consensus messages.

Clean interfaces keep Phase B (parallel, DewTx) from rewriting the whole node.
