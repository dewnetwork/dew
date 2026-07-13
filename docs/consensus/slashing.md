---
title: Slashing
description: Penalties for double-signing and downtime.
category: consensus
order: 30
status: stable
---

# Slashing

Slashing aligns security with stake. **Penalty percentages below remain tentative** (economics review / mainnet). On-chain today: **double-sign evidence is verified** (dual signed prevote/precommit at same height/round, distinct hashes) then jail via `0x102` method `0x06`. Stake burn percentages still not applied on-chain.

## Offenses

### 1. Double-signing (equivocation)

Signing two different proposals or votes for the same `(height, round, vote-type)`.

**Severity: critical**

| Action               | Tentative penalty                                                        |
| :------------------- | :----------------------------------------------------------------------- |
| Validator self-stake | Up to **100% burn** + permanent jail                                     |
| Delegators           | Small correlated penalty (e.g. **5%**) — incentivizes careful delegation |
| Evidence             | Included on-chain; gossiped as `Evidence` messages                       |

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
