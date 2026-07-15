---
title: Slashing
description: Penalties for double-signing and downtime.
category: consensus
order: 30
status: stable
---

# Slashing

Slashing aligns security with stake. **Double-sign burn percentages are testnet provisional** in `params` (economics may re-freeze before mainnet). On-chain today: **double-sign evidence is verified** (dual signed prevote/precommit at same height/round, distinct hashes) then **`SlashAndJail`** via `0x102` method `0x06` — burns stake then jails.

Design: [0x102 reward + slash](../superpowers/specs/2026-07-15-0x102-reward-slash-design.md).

## Offenses

### 1. Double-signing (equivocation)

Signing two different proposals or votes for the same `(height, round, vote-type)`.

**Severity: critical**

| Action               | On-chain (testnet provisional)                                           |
| :------------------- | :----------------------------------------------------------------------- |
| Validator self-stake | **100% burn** (`DoubleSignSelfBurnBps=10000`) + permanent jail           |
| Delegators           | **5%** effective delegated burn (`DoubleSignDelegatorBurnBps=500`) via exchange rate |
| Evidence             | Dual-vote wire on `0x06`; burned wei removed from `0x102` escrow         |

### 2. Downtime

Missing a large fraction of expected signs in a window (e.g. **≥ 50% misses in 1,000 blocks**).

**Severity: mild**

| Action     | Tentative penalty                                       |
| :--------- | :------------------------------------------------------ |
| Jail       | Temporary (e.g. 2 hours wall time or equivalent blocks) |
| Stake burn | Small (e.g. **0.01%**)                                  |
| Recovery   | Submit `unjail` after node is healthy                   |

## Evidence handling

1. Any node can broadcast evidence of conflicting signatures
2. Light verification: same validator key, same height/round, different block hashes
3. Once included/accepted, apply slash and update validator set

## Design principles

- Prefer **objective** faults (signatures) over subjective ones
- Never slash for honest network partition without clear protocol rule
- Publish exact percentages in genesis or a governance-controlled param store before mainnet
