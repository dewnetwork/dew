---
title: Precompiles
description: Standard Ethereum precompiles and future Dew system contracts.
category: execution
order: 40
status: draft
---

# Precompiles

Precompiles are native implementations exposed at fixed addresses, callable like contracts but executed in Go.

## Phase A — standard Ethereum set

| Address | Name                                      |
| :------ | :---------------------------------------- |
| `0x01`  | ecrecover                                 |
| `0x02`  | SHA2-256                                  |
| `0x03`  | RIPEMD-160                                |
| `0x04`  | identity                                  |
| `0x05`  | modexp                                    |
| `0x06`  | ecAdd (alt_bn128)                         |
| `0x07`  | ecMul                                     |
| `0x08`  | ecPairing                                 |
| `0x09`  | blake2f                                   |
| `0x0a`  | point evaluation (KZG / EIP-4844 related) |

Gas costs: match Cancun unless a documented exception exists.

## Phase B — Dew system precompiles (_planned_)

Reserved starting at `0x100`:

| Address | Name                    | Purpose                        |
| :------ | :---------------------- | :----------------------------- |
| `0x101` | Native swap / orderbook | High-performance market ops    |
| `0x102` | Staking entrypoint      | Stake / delegate from Solidity |

These call into native modules; they are **not** required for Phase A ERC-20 workflows.

### Design rules for custom precompiles

1. Fixed gas schedule (no unbounded native work free of gas)
2. Deterministic output for given input + state
3. Document ABI / byte layout before any testnet uses them
4. Prefer fail-safe reverts over panics in Go

## Implementation

Register precompiles in the EVM config used by the executor. Keep custom addresses out of the active map until Phase B feature flags enable them.
