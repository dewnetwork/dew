---
title: EVM Integration
description: Embedding the Ethereum Virtual Machine in the Dew node.
category: execution
order: 20
status: draft
---

# EVM Integration

## Approach

Dew embeds a standard EVM interpreter (typically via `go-ethereum` `core/vm`) behind a thin adapter. Opcode semantics follow the **Cancun-era** rules enabled from genesis (see [Genesis](../economics/genesis.md)).

```
┌─────────────────────────────────────┐
│         Dew Executor           │
└─────────────────┬───────────────────┘
                  │
                  ▼
┌─────────────────────────────────────┐
│     StateDB bridge (interface)      │
│   Flat DB + journal + gas metering  │
└─────────────────┬───────────────────┘
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
   EVM opcodes        Precompiles
```

## StateDB bridge

Implement go-ethereum’s `vm.StateDB` (or equivalent) mapping:

| Method                                     | Backend               |
| :----------------------------------------- | :-------------------- |
| `GetBalance` / `AddBalance` / `SubBalance` | Account in flat state |
| `GetNonce` / `SetNonce`                    | Account nonce         |
| `GetCode` / `SetCode`                      | Code store            |
| `GetState` / `SetState`                    | Storage DB            |
| `Snapshot` / `RevertToSnapshot`            | Journal               |
| `AddLog`                                   | Receipt logs          |
| `Exist` / `Empty` / `CreateAccount`        | Account lifecycle     |
| `Selfdestruct` / `HasSelfdestructed`       | Per Cancun rules      |

## Block context

Provide block number, timestamp, coinbase/proposer, gas limit, base fee, randomness if required by the fork rules.

## Contract deployment

1. `to == nil`
2. `data` = init code
3. EVM runs init code; returned bytes = runtime code
4. Address via CREATE / CREATE2 rules
5. Store code; set `CodeHash`

## Testing strategy

- Unit: individual opcodes via state tests if imported
- Integration: deploy ERC-20, transfer, approve, emit events
- Compare receipts and storage against a reference geth node on the same genesis **where opcodes overlap**

## What we do not fork early

Avoid custom opcodes in Phase A. Performance comes from storage layout, block time, gas schedule, and later parallel scheduling — not a divergent VM dialect.
