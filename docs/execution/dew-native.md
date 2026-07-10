---
title: Dew-Native Execution
description: Phase B native transaction path and system modules.
category: execution
order: 60
status: draft
---

# Dew-Native Execution

> **Phase B.** Optional high-performance path alongside EVM — not a replacement for Solidity support.

## Why native?

| EVM path               | Native path                |
| :--------------------- | :------------------------- |
| General programs       | Fixed operations / modules |
| Interpreter overhead   | Compiled Go                |
| Dynamic storage access | Declared `AccessList`      |
| Gas per opcode         | Flat / predictable fee     |

Use cases: micro-payments, games, orderbook updates, internal system calls.

## Transaction type

See [Transactions](../protocol/transactions.md) for `DewTx` fields.

Pipeline:

```
dew_sendRawTransaction
    → decode + verify domain-separated signature
    → mempool (may be separate or unified)
    → block inclusion (tagged union)
    → native executor (not EVM)
    → state writes + receipt-like result
```

## Access lists and parallelism

`AccessList` is **mandatory** for `DewTx`. The scheduler assigns txs with disjoint address sets to different workers without optimistic abort (if lists are complete). Incomplete lists that touch undeclared accounts **must fail** the tx (fail closed), not silently escalate privileges.

## Native modules (examples, not commitments)

| Module                | Address space       | Notes                        |
| :-------------------- | :------------------ | :--------------------------- |
| System token transfer | built-in            | Balance move without EVM     |
| Staking               | precompile / native | Align with consensus staking |
| DEX / orderbook       | precompile `0x101`  | Later product surface        |

Each module needs: input encoding, auth model, fee, and state keys documented before enablement.

## Compatibility promise

- Solidity apps keep using EVM forever.
- Native is opt-in via new RPC and client support.
- Bridges between EVM contracts and native modules use precompiles with explicit ABIs.
