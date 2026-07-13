---
title: Precompiles
description: Standard Ethereum precompiles and Dew system contracts.
category: execution
order: 40
status: stable
---

# Precompiles

Precompiles are native implementations exposed at fixed addresses, callable like contracts but executed in Go.

## Standard Ethereum set (Cancun)

| Address | Name |
| :--- | :--- |
| `0x01` | ecrecover |
| `0x02` | SHA2-256 |
| `0x03` | RIPEMD-160 |
| `0x04` | identity |
| `0x05` | modexp |
| `0x06` | ecAdd (alt_bn128) |
| `0x07` | ecMul |
| `0x08` | ecPairing |
| `0x09` | blake2f |
| `0x0a` | point evaluation (KZG / EIP-4844 related) |

Gas costs: match Cancun unless a documented exception exists.

## Dew system precompiles

Reserved starting at `0x100`. Enabled when the executor feature flag is on
(`Executor.EnableDewPrecompiles(true)`, default on; matches `params.DefaultEnableDewPrecompiles`).

| Address | Name | Gas (fixed) | Status |
| :--- | :--- | ---: | :--- |
| `0x100` | Native transfer | 3_000 | **Active** — forward CALLVALUE to recipient |
| `0x101` | Native swap / book | TBD | Reserved |
| `0x102` | Staking entrypoint | method-based | **C4** — bond/unbond/queries/jail; feature-flagged |

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

### `0x102` — Staking entrypoint (Phase C4)

Enabled when Dew precompiles are on **and** `Executor.EnableStaking(true)` / `Node.SetStakingEnabled(true)`. Default **off** until operators opt in (`params.DefaultEnableStaking = false`).

**Byte layout** (fail-closed; not full Solidity ABI):

| Method | Input | Value | Gas | Effect |
| :--- | :--- | :--- | --: | :--- |
| `0x00` bond | `[0x00]` | self-stake amount | 50_000 | Escrow CALLVALUE on `0x102`; credit tx sender |
| `0x01` unbond | `[0x01 \|\| amount uint256]` | 0 | 40_000 | Reduce stake; queue amount until unbonding period |
| `0x02` getSelfStake | `[0x02 \|\| addr20]` | 0 | 2_000 | Return stake uint256 |
| `0x03` getVotingPower | `[0x03 \|\| addr20]` | 0 | 2_000 | 0 if jailed / not candidate |
| `0x04` activeCount | `[0x04]` | 0 | 2_000 | Top-K set size |
| `0x05` activeAt | `[0x05 \|\| index uint256]` | 0 | 2_000 | Address at rank |
| `0x06` jail | `[0x06 \|\| addr20 \|\| evidenceHash32]` | 0 | 30_000 | Jail (non-zero evidence required) |
| `0x07` isJailed | `[0x07 \|\| addr20]` | 0 | 2_000 | 0/1 |
| `0x08` withdraw | `[0x08]` | 0 | 40_000 | Claim matured unbond to sender (D3c) |
| `0x09` pendingUnbond | `[0x09 \|\| addr20]` | 0 | 2_000 | `amount` (32) \|\| `unlockAt` unix (32) |

**Rules:** min self-stake `100_000 * 10^18` wei (**public-testnet-v1**); active set = top `K` (default 100) by voting power among candidates ≥ min and not jailed. **Unbonding** uses block timestamp + genesis `unbondingPeriodSeconds` (default 604_800); funds stay at `0x102` until `withdraw` after unlock. Bond credits **tx sender** (EOA path); nested contract staking deferred. See [Public testnet freeze](../ops/public-testnet.md) and [D3 scale — D3c](../scale/d3-scale.md).

Module state: `core/native/staking.go` storage under address `0x102`.

### Design rules for custom precompiles

1. Fixed gas schedule (no unbounded native work free of gas)
2. Deterministic output for given input + state
3. Document ABI / byte layout before any testnet uses them
4. Prefer fail-safe reverts over panics in Go

## Implementation

`core/vm/precompiles.go` clones Cancun precompiles and registers Dew addresses via `evm.SetPrecompiles` when the flag is enabled. Keep inactive addresses out of the map when the flag is off.
