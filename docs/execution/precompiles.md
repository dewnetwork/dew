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

## Phase B — Dew system precompiles

Reserved starting at `0x100`. Enabled when the executor feature flag is on
(`Executor.EnableDewPrecompiles(true)`, default on; matches `params.DefaultEnableDewPrecompiles`).

| Address | Name               | Gas (fixed) | Status                                      |
| :------ | :----------------- | ----------: | :------------------------------------------ |
| `0x100` | Native transfer    |       3_000 | **Active** — forward CALLVALUE to recipient |
| `0x101` | Native swap / book |         TBD | Reserved                                    |
| `0x102` | Staking entrypoint |       2_000 | Reserved stub (reverts until staking lands) |

### `0x100` — Native transfer

Useful bridge from Solidity into native DEW movement without an ERC-20 hop.

**Call convention**

- `to` = `0x0000…0100`
- `value` = amount of native DEW to forward
- `data` = 20-byte recipient address (exactly 20 bytes)

**Semantics**

1. EVM transfers `value` from caller to `0x100` (standard CALL value rules).
2. Precompile moves the full balance of `0x100` to `recipient`.
3. Returns `uint256` amount forwarded (ABI left-padded 32 bytes).

**Rules:** fixed gas; deterministic; reverts on malformed input (not 20 bytes). Feature flag off → address is a normal empty account (value sits at `0x100`, not forwarded).

### Design rules for custom precompiles

1. Fixed gas schedule (no unbounded native work free of gas)
2. Deterministic output for given input + state
3. Document ABI / byte layout before any testnet uses them
4. Prefer fail-safe reverts over panics in Go

## Implementation

`core/vm/precompiles.go` clones Cancun precompiles and registers Dew addresses via `evm.SetPrecompiles` when the flag is enabled. Keep inactive addresses out of the map when the flag is off.
