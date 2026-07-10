---
title: Roadmap
description: Phased path from monorepo scaffold to ETH-compatible devnet and Dew advantages.
category: development
order: 20
status: draft
---

# Roadmap

All phases land in the **same monorepo** (Go core + Node for **web docs build** and tooling). See [Monorepo layout](./go-project-layout.md).

## Strategy

```mermaid
flowchart TB
  subgraph A["Phase A — ETH-compatible L1 first"]
    direction LR
    A1[Crypto] --> A2[Types/DB] --> A3[EVM] --> A4[RPC] --> A5[BFT] --> A6[P2P] --> A7[Devnet]
  end
  subgraph B["Phase B — faster / cheaper / native"]
    direction LR
    B1[Dew-PE] --> B2[DewTx] --> B3[Modules] --> B4[Precompiles / tune]
  end
  A --> B
```

**Ship A before optimizing B.** Parallel execution on a broken sequential chain only multiplies bugs.

## Phase A detail

```mermaid
flowchart TD
  A1[A1 Cryptography & dewcli wallet] --> A2[A2 Types, DB, flat state]
  A2 --> A3[A3 EVM bridge & sequential execution]
  A3 --> A4[A4 JSON-RPC MetaMask / Foundry]
  A4 --> A5[A5 Dew-BFT local]
  A5 --> A6[A6 P2P gossip & sync]
  A6 --> A7[A7 Multi-validator devnet + Solidity]
```

## Phase B detail

```mermaid
flowchart TD
  B1[B1 Dew-PE Block-STM behind Executor] --> B2[B2 DewTx + dew_sendRawTransaction]
  B2 --> B3[B3 System precompiles / native modules]
  B3 --> B4[B4 Load tests, fee tuning, security audit]
```

## Mapping to docs

| Phase | Primary docs                                                                                                |
| :---- | :---------------------------------------------------------------------------------------------------------- |
| A1    | [Cryptography](../protocol/cryptography.md), [Addresses](../protocol/addresses.md)                          |
| A2    | [State](../protocol/state.md), [Blocks](../protocol/blocks.md), [Transactions](../protocol/transactions.md) |
| A3    | [EVM](../execution/evm.md), [Gas](../execution/gas-and-fees.md)                                             |
| A4    | [JSON-RPC](../api/json-rpc.md)                                                                              |
| A5    | [Dew-BFT](../consensus/dew-bft.md), [Validators](../consensus/validators.md)                                |
| A6    | [P2P](../networking/p2p.md), [Gossip and sync](../networking/gossip-and-sync.md)                            |
| A7    | [Genesis](../economics/genesis.md), [Ethereum compatibility](../overview/ethereum-compatibility.md)         |
| B1    | [Parallel execution](../execution/parallel-execution.md)                                                    |
| B2–B3 | [Dew-native](../execution/dew-native.md), [Dew RPC](../api/dew-extensions.md)                               |

## Milestone definition of done

See [Phases](./phases.md) for acceptance criteria per phase.
