---
title: Gas and Fees
description: Gas schedule, block limits, and EIP-1559 fee market.
category: execution
order: 30
status: draft
---

# Gas and Fees

Goals: **cheaper than Ethereum mainnet for users**, while retaining gas as the metering unit for EVM safety (halting problem / DoS).

## Gas metering

- Every EVM opcode consumes gas per the active schedule.
- Phase A baseline: **Ethereum Cancun schedule** as default.
- Optional Dew discount (_tentative_): `SLOAD` / `SSTORE` costs reduced (e.g. up to 50%) reflecting flat DB — must be a **fixed table**, not ad hoc.

Intrinsic gas for a simple transfer remains **21,000** unless a documented hardfork changes it.

## Block gas limit

| Parameter                     | Tentative value | Notes                                                                |
| :---------------------------- | :-------------- | :------------------------------------------------------------------- |
| `gasLimit`                    | `120_000_000`   | Higher than Ethereum; enabled by faster blocks + later parallel exec |
| Target utilization (EIP-1559) | 50% of limit    | Base fee adjustment                                                  |

## EIP-1559 fee market

$$
\text{txFee} = \text{gasUsed} \times (\text{baseFee} + \text{priorityFee})
$$

| Component              | Behavior                                                                                 |
| :--------------------- | :--------------------------------------------------------------------------------------- |
| **Base fee**           | Protocol-adjusted each block; **burned** (removed from supply)                           |
| **Priority fee (tip)** | User-selected; paid to **block proposer** (or validator reward pool — freeze one policy) |

Base fee update rule: follow Ethereum’s elasticity / denominator constants unless a Dew-specific config is set in genesis.

## Why cheaper than ETH (mechanism, not marketing)

1. More block space per wall-clock second (1s blocks × high gas limit)
2. Lower storage opcode costs (flat state)
3. Later: parallel execution increases effective capacity
4. Later: Dew-native micro-fees for non-EVM actions

Burning base fee still creates deflationary pressure under high load.

## Dew-native fees (Phase B)

`DewTx` uses a **flat fee in wei** (not EVM gas), defined in protocol/genesis. Directionally ~10% of a simple transfer’s effective cost — exact constant TBD before freeze.

## Header fields

`BaseFee`, `GasLimit`, `GasUsed` are first-class header fields. See [Blocks](../protocol/blocks.md).
