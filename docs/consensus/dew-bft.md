---
title: Dew-BFT
description: Byzantine fault tolerant consensus protocol for Dewchain.
category: consensus
order: 10
status: draft
---

# Dew-BFT

Dew-BFT is a **Proof-of-Staked Authority (PoSA)** BFT engine: bonded validators vote by stake-weighted power; commits are **final**.

## Goals

| Goal                     | Mechanism                          |
| :----------------------- | :--------------------------------- |
| Faster than ETH finality | 1-block finality on commit         |
| Safety                   | \(> 2/3\) voting power quorums     |
| Liveness                 | Round timeouts + proposer rotation |
| Accountability           | Slashing on double-sign            |

## Parameters (_tentative_)

| Parameter             | Value                          |
| :-------------------- | :----------------------------- |
| Block time target     | 1s                             |
| Fault tolerance       | \(N \ge 3F + 1\)               |
| Quorum                | \(> 2/3\) voting power         |
| Active set size \(K\) | 21 (testnet); configurable     |
| Epoch length          | 86,400 **blocks** (~24h at 1s) |

## Round state machine

For each height \(H\), rounds \(R = 0, 1, \ldots\):

```
NewRound → Propose → Prevote → Precommit → Commit
                ▲                  │
                └──── timeout ─────┘
```

### Propose

- Proposer for \((H, R)\) chosen by **stake-weighted round-robin** (deterministic; same on all honest nodes).
- Proposer packs mempool txs, executes (or uses speculative execution), fills roots + `BaseFee`, signs proposal, broadcasts `Proposal`.

### Prevote

- Validate header linkage, signatures, gas fields, full execution → expected roots.
- Valid → prevote for block hash; else prevote `nil`.
- Wait for \(> 2/3\) power of prevotes (block or nil) or timeout.

### Precommit

- If \(> 2/3\) prevotes for block \(B\) → precommit \(B\).
- Else → precommit `nil`, advance to round \(R+1\) after timeout rules.
- Wait for \(> 2/3\) precommits.

### Commit

- On \(> 2/3\) precommits for \(B\): persist block, apply state if not already, store quorum certificate, set height \(H+1\), round 0.

## Locking (Tendermint-style)

To prevent safety bugs across rounds, validators SHOULD implement **PoLC locking**: once precommitting \(B\) after a polka, do not prevote a conflicting block at the same height unless a newer polka justifies unlock. Full lock rules to be mirrored from Tendermint/CometBFT semantics before mainnet.

## Consensus messages (wire)

These are **separate** from block/tx gossip types:

| Message                     | Purpose                               |
| :-------------------------- | :------------------------------------ |
| `Proposal`                  | Block (or hash + parts) for \((H,R)\) |
| `Prevote`                   | Vote for hash or nil                  |
| `Precommit`                 | Vote for hash or nil                  |
| `NewRoundStep` / heartbeats | Optional coordination                 |
| `Evidence`                  | Double-sign proofs                    |

Transport: same P2P framing as [Networking](../networking/p2p.md), distinct message type IDs.

## Empty blocks

If the mempool is empty, proposers may still propose an empty block so height advances and finality continues.

## Full nodes

Non-validators execute committed blocks (after receiving commit cert + block) and update state. They do not vote.
