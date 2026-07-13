---
title: Dew-Native Execution
description: Native transaction path and system modules alongside EVM.
category: execution
order: 60
status: stable
---

# Dew-Native Execution

> Optional high-performance path alongside EVM — not a replacement for Solidity support.

## Why native?

| EVM path | Native path |
| :--- | :--- |
| General programs | Fixed operations / modules |
| Interpreter overhead | Compiled Go |
| Dynamic storage access | Declared `AccessList` |
| Gas per opcode | Flat / predictable fee |

Use cases: micro-payments, games, orderbook updates, internal system calls.

## Transaction type

See [Transactions](../protocol/transactions.md) for `DewTx` fields and wire encoding (`0xdf`).

Pipeline (public-testnet-v1):

```
dew_sendRawTransaction
    → decode + verify domain-separated signature
    → unified mempool (C1 admission + gap queue)
    → auto-mine pack or BFT proposal path
    → native executor (core/native)
    → state writes + receipt / tx index
```

**Body note:** under this freeze the EVM block body does not carry a tagged-union DewTx list. Native inclusion is applied and indexed even when the body is empty; see [Blocks](../protocol/blocks.md).

## Access lists and parallelism

`AccessList` is **mandatory** for modules that touch extra accounts (e.g. secondary credit). Incomplete lists that touch undeclared accounts **must fail** the tx (fail closed), not silently escalate privileges. Sender, receiver, and the protocol fee sink are always permitted.

## Native modules

| Module | Surface | Status |
| :--- | :--- | :--- |
| System token transfer | DewTx empty payload | **Active** (`core/native`) |
| Secondary credit | DewTx payload `0x01` | **Active** — requires AccessList |
| Native transfer (EVM) | precompile `0x100` | **Active** — CALLVALUE forward |
| Staking | precompile `0x102` | **Active** (flag off by default) — residuals in D3c |
| DEX / orderbook | precompile `0x101` | Reserved |

Each new module needs: input encoding, auth model, fee, and state keys documented before enablement.

## Fees

Flat fee `params.DefaultDewTxFeeWei` / min floor — [Gas and fees](./gas-and-fees.md). Paid to block proposer.

## Multi-tx auto-mine

Ready continuous DewTx nonce chains pack like EVM (up to 64 per seal) with the same gap-queue rules. See C1 notes in [Phases](../build/phases.md#c1--mempool-admission--fee-policy).

## Compatibility promise

- Solidity apps keep using EVM forever.
- Native is opt-in via `dew_*` RPC and client support.
- Bridges between EVM contracts and native modules use precompiles with explicit layouts ([Precompiles](./precompiles.md)).
