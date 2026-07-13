---
title: Research lab harness
description: Track R hypothesis, reproducible PE/load commands, and S1 baseline numbers.
category: ops
order: 55
status: stable
---

# Research lab harness

Short lab note for **Track R** / plan **S1**. Goal: one written hypothesis, commands anyone can re-run, and a recorded baseline. Not marketing; numbers vary by machine.

**Related:** [Parallel execution](../execution/parallel-execution.md) · [Phases — Track R](../build/phases.md#track-r--research-lab) · [Plan S0–S6](../build/phases.md#recommended-sequence--research-lab--precompile-slots) · package `tests/load/`, `core/vm`.

---

## Hypothesis (S1)

**H1 — PE wall-clock vs conflict structure (fork+overlay)**

On the current Dew-PE path (`core/vm/parallel.go` fork + overlay, not full Block-STM):

1. For **disjoint simple value transfers**, parallel wall-clock is often **slower** than sequential (`speedup < 1`) because per-tx fork/overlay cost dominates ~21k-gas work.
2. **Serial equivalence** still holds: PE and sequential produce the same state root (and receipt shape) on non-conflicting and mixed-conflict fixtures.
3. On a **mixed hub workload** (~25% txs touch a shared account), PE records a **non-zero rollback / conflict rate** while roots still match sequential.
4. **Implication for S2 / Track 4:** full Block-STM is **not** justified by simple-transfer speedup alone; only pursue if a heavier non-conflicting matrix (or multi-contract load) shows capacity gains that fork+overlay cannot deliver without excessive rollbacks.

Falsifiers for later runs:

- Disjoint simple transfers consistently `speedup ≥ 1.5×` on this machine class → re-check overhead assumptions.
- Mixed roots diverge → treat as freeze-severity PE bug, not research.
- Zero rollbacks on mixed hub fixture → harness no longer stresses shared keys.

---

## Harness commands

Prereq: Go 1.25+ from monorepo root. No live RPC required for PE load tests.

### Lab baseline (S0 residual / preflight)

```bash
go test ./... -count=1
go test ./devnet/ -count=1
```

### PE load + metrics (primary S1 harness)

```bash
# Verbose so t.Logf speedup / rollback lines print
go test ./tests/load/ -count=1 -timeout 120s -v

# PE unit path (equivalence + metrics)
go test ./core/vm/ -count=1 -timeout 60s -run 'Parallel' -v
```

Optional RPC stats after a local node has run PE blocks:

```bash
# with dew devnet or dew run listening
curl -s -X POST http://127.0.0.1:8545 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"dew_getExecutionStats","params":[]}'
```

See [dew_getExecutionStats](../api/dew-extensions.md#dew_getexecutionstats).

### What to capture

| Field | Source |
| :--- | :--- |
| `sequential` / `parallel` duration, `speedup` | `TestLoad_ParallelVsSequential_NonConflicting` log |
| `rollbacks`, `conflict_rate` | load + PE tests `Stats()` |
| root match | test pass/fail (hard gate) |
| workers | log line / `GOMAXPROCS` |

S2 will expand this into a conflict-rate × worker matrix; this note only needs **one** reproducible baseline.

---

## Baseline run (S1)

| | |
| :--- | :--- |
| **Date** | 2026-07-13 |
| **Host** | macOS (local developer machine) |
| **Command** | `go test ./tests/load/ -count=1 -timeout 120s -v` and `go test ./core/vm/ -count=1 -run 'Parallel' -v` |
| **Result** | all PASS |

### `tests/load` (excerpt)

| Case | n | Workers | Sequential | Parallel | Speedup | Rollbacks | Conflict rate |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Non-conflicting transfers | 128 | 8 | 3.03 ms | 5.28 ms | **0.57×** | 0 | 0.000 |
| Mixed hub conflicts | 64 | default | — (equiv only) | — | — | **15** | **0.234** |
| Native DewTx throughput | 200 | — | — | — | — | — | ~33k tps (in-process) |

Notes from harness comments: for tiny EVM transfers, fork+overlay overhead can exceed sequential wall time; **correctness + speculative commit rate** are the hard gates. Speedup is expected mainly on heavier non-conflicting contract work (S2 matrix).

### `core/vm` Parallel (excerpt)

| Case | Notes |
| :--- | :--- |
| Equivalence non-conflicting / conflicting | PASS |
| Speedup non-conflicting | seq ≈ 1.36 ms, par ≈ 2.56 ms, rollbacks=0, conflictRate=0, workers=8 (speedup &lt; 1, consistent with H1) |
| Metrics recorded | PASS |

### Reading for H1

- **Supported so far:** (1) simple disjoint PE slower wall-clock; (2) roots match; (3) mixed conflicts produce ~23% conflict rate with equivalence.
- **Not yet measured (S2):** conflict-rate sweep, worker sweep, heavier contract workloads, multiproc BFT commit latency.

---

## Next (S2)

- Build a small matrix: sequential vs PE × {disjoint, mixed, high-conflict} × worker counts.
- Keep serial-equivalent tests green; file Block-STM only if numbers demand it.
- Optional multiproc soak notes remain a separate Track R focus (BFT), not required to close S1.
