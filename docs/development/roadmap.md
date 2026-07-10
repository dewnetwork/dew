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

```
Phase A — Ethereum-compatible L1 (priority)
   Crypto → Types/DB → EVM → RPC → BFT → P2P → Devnet
   (+ Node scripts / RPC smoke tests as the node becomes reachable)

Phase B — Faster / cheaper / native (after A works)
   Dew-PE parallel → DewTx → Native modules → Precompiles
   (+ optional TS SDK under packages/)
```

**Ship A before optimizing B.** Parallel execution on a broken sequential chain only multiplies bugs.

## Phase A detail

```
┌────────────────────────────────────────────┐
│ A1 Cryptography & dewcli wallet            │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ A2 Types, DB, flat state                   │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ A3 EVM bridge & sequential execution       │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ A4 JSON-RPC (MetaMask / Foundry)           │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ A5 Dew-BFT (single / multi process local)  │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ A6 P2P gossip & sync                       │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ A7 Multi-validator devnet + Solidity apps  │
└────────────────────────────────────────────┘
```

## Phase B detail

```
┌────────────────────────────────────────────┐
│ B1 Dew-PE (Block-STM) behind Executor API  │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ B2 DewTx + dew_sendRawTransaction          │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ B3 System precompiles / native modules     │
└───────────────────┬────────────────────────┘
                    ▼
┌────────────────────────────────────────────┐
│ B4 Load tests, fee tuning, security audit  │
└────────────────────────────────────────────┘
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
