---
title: Architecture Overview
description: High-level node architecture and data flow for Dew.
category: architecture
order: 10
status: stable
---

# Architecture Overview

Dew is a **monorepo**: the **Go** process is the chain node; **Node.js** builds the **docs website** and product SPAs and does **not** run consensus. See [Monorepo layout](../build/go-project-layout.md).

## Node at a glance

A full Dew node is a single Go process that:

1. Accepts transactions (RPC / P2P)
2. Holds them in a mempool
3. Participates in Dew-BFT (if validator) or follows commits (if full node)
4. Executes transactions against state
5. Persists blocks and state under `<datadir>/chaindata` (Pebble)
6. Serves JSON-RPC (HTTP) to wallets and dapps

```mermaid
flowchart LR
  W[Wallets / tools] --> RPC[JSON-RPC HTTP]
  Peers[Peers] <--> P2P[P2P]
  RPC --> MP[Mempool]
  P2P --> MP
  MP --> C[Consensus Dew-BFT]
  C --> EX[Execution EVM / DewTx]
  EX --> DB[(Pebble chaindata)]
  P2P <--> C
  RPC -.-> DB
```

## Component map

| Component | Package | Responsibility |
| :--- | :--- | :--- |
| CLI / node entry | `cmd/dew`, `cmd/dewcli`, `cmd/dewfaucet` | Process bootstrap, wallet, ops faucet |
| Types | `core/types` | Block, header, tx, receipt, DewTx |
| State | `core/state` | Flat KV hot path + SMT commit roots |
| VM | `core/vm` | EVM bridge, PE, precompiles |
| Native | `core/native` | DewTx executor, staking |
| Crypto | `crypto` | Keys, sign, verify, hashes |
| Database | `db` | **Pebble only** (`db.PebbleDB`) |
| Mempool | `mempool` | Unified admission for EVM + DewTx |
| Node backend | `node` | Genesis open, seal/import, Stack, chaindata |
| Consensus | `consensus` | Dew-BFT engine, builder, runner |
| P2P | `p2p` | Peers, gossip, sync, encrypted transport |
| RPC | `rpc` | HTTP JSON-RPC (`eth_*` / `net_*` / `web3_*` / `dew_*`) |
| Config / params | `config`, `params` | Genesis + chain constants / freeze |
| Ops faucet | `faucet` | Rate-limited drip (not consensus) |

## Critical data paths

### A. User transaction

```mermaid
sequenceDiagram
  participant Client
  participant RPC as JSON-RPC
  participant MP as Mempool
  participant P2P
  participant Prop as Proposer
  participant Vals as Validators
  participant DB as Pebble chaindata

  Client->>RPC: eth_sendRawTransaction
  RPC->>RPC: decode RLP + verify ECDSA + chainId
  RPC->>MP: admit (nonce / fee floors / size)
  MP->>P2P: gossip Inventory / Tx
  Prop->>MP: pull txs into block
  Prop->>Vals: Proposal (header + body)
  Vals->>Vals: execute (sequential or PE-equivalent)
  Vals->>Vals: StateRoot + ReceiptRoot match
  Vals->>Vals: Prevote → Precommit → Commit
  Vals->>DB: ImportCommittedBlock + persist batch
```

### B. State commitment (normative)

Validators must agree on `StateRoot` at commit time:

1. Execute all txs in the block
2. Compute SMT `StateRoot` (and tx/receipt roots)
3. Header must contain those roots before votes that finalize the block

Flat KV remains the execution hot path; SMT is commit-time only. See [State](../protocol/state.md).

### C. Parallel execution (performance layer)

```mermaid
flowchart TD
  MP[Mempool] --> Hints[Optional access-list hints]
  Hints --> PE[Dew-PE workers + overlay]
  PE --> Val{Validate read sets}
  Val -->|conflict| Re[Re-execute Ti + dependents]
  Re --> Val
  Val -->|ok| Root[Same StateRoot as sequential]
```

Parallel execution must not change the post-state root consensus votes on.

## Deployment roles

| Role | RPC | P2P | Propose | Vote |
| :--- | :---: | :---: | :---: | :---: |
| Validator (`--validator`) | optional | yes | when selected | yes |
| Full node (sync) | yes | yes | no | no (follows commits) |
| Dev / Path B auto-mine | yes | optional | pack ready pending (≤64) | n/a |

```mermaid
flowchart TB
  subgraph Validators
    V1[Validator A]
    V2[Validator B]
    V3[Validator C]
  end
  FN[Full / RPC node] --> V1
  FN --> V2
  FN --> V3
  Apps[Wallets / dapps] --> FN
```

Multi-process BFT: [D3 scale](../scale/d3-scale.md). Disk layout: [Durable chaindata](../ops/durable-chaindata.md).

## Design boundaries

- **Consensus does not embed EVM logic** — it calls execution via builder / import path.
- **RPC does not talk to DB bypassing state layer** — queries go through chain/state APIs.
- **P2P does not interpret contract ABI** — only tx bytes, blocks, and consensus messages.
- **Peers are not in chaindata** — `<datadir>/peers.json` has a separate lifecycle.
