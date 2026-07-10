---
title: Parallel Execution (Dew-PE)
description: Phase B optimistic parallel execution based on Block-STM ideas.
category: execution
order: 50
status: draft
---

# Parallel Execution (Dew-PE)

> **Phase B.** Implement only after sequential EVM + multi-validator devnet are correct.  
> Normative semantics: **equivalent to sequential execution in transaction index order.**

## Motivation

Sequential EVM leaves CPU cores idle. Dew-PE runs non-conflicting transactions concurrently using Go goroutines, then validates consistency.

## Model: optimistic Block-STM style

Dewchain does **not** require users to declare access lists for normal EVM txs (lists are often wrong for dynamic contracts). Instead:

1. Execute optimistically in parallel
2. Record read/write sets in an MVCC cache
3. Validate; re-execute on conflict
4. Commit a final state identical to serial order \(T_0, T_1, \ldots\)

```
Block txs [T0..Tn]
        │
        ▼
  Worker pool (goroutines)
        │
        ▼
  MVCC read/write sets
        │
        ▼
  Validate by increasing index
        │
   conflict? ──yes──▶ abort write set, re-execute Ti (+ dependents)
        │ no
        ▼
  Merge → StateRoot
```

## MVCC cache

For each tx index \(i\):

- **Read set**: keys + version (which earlier tx index produced the value)
- **Write set**: pending account/storage updates

Read rule: value from the highest \(j < i\) that wrote the key, else committed parent state.

## Validation

For each \(T_i\) in order:

- If any key in the read set was written by a later-than-observed version from some \(T_j\) with \(j < i\) after \(T_i\)’s read → **invalid**
- On invalid: drop \(T_i\) writes; mark dependent txs invalid; re-run

## Determinism

Given the same parent state and the same ordered tx list, Dew-PE MUST produce the same receipts (gas, status, logs) and `StateRoot` as sequential Apply.

## Relationship to access lists

| Tx kind         | Scheduling                                                                               |
| :-------------- | :--------------------------------------------------------------------------------------- |
| EVM tx          | Optimistic STM (access list optional hint only)                                          |
| DewTx (Phase B) | **Static** partition by declared `AccessList` — can skip abort path when lists are exact |

This resolves the old doc conflict: **spec-level primary model for EVM = optimistic**; **DewTx = declared dependencies**.

## Complexity controls

- Cap worker count to `GOMAXPROCS` / config
- Bound re-execution attempts; fall back to sequential for pathological blocks
- Metrics: rollback rate, worker utilization (`dew_getExecutionStats`)

## Non-goals for first Dew-PE

- Cross-block speculative execution
- GPU interpreters
- Breaking receipt order vs tx index
