---
title: Research lab harness
description: Track R hypothesis, PE load matrix commands, and S1–S2 baseline numbers.
category: ops
order: 55
status: stable
---

# Research lab harness

Lab note for **Track R** / plan **S0–S2**. One written hypothesis, reproducible commands, and recorded baselines. Not marketing; wall-clock numbers vary by machine.

**Related:** [Parallel execution](../execution/parallel-execution.md) · [Phases — Track R](../build/phases.md#track-r--research-lab) · [Plan S0–S6](../build/phases.md#recommended-sequence--research-lab--precompile-slots) · package `tests/load/`, `core/vm`.

---

## Hypothesis (S1)

**H1 — PE wall-clock vs conflict structure (fork+overlay)**

On the current Dew-PE path (`core/vm/parallel.go` fork + overlay, not full Block-STM):

1. For **disjoint simple value transfers**, multi-worker parallel wall-clock is often **slower** than sequential (`speedup < 1`) because per-tx fork/overlay cost dominates ~21k-gas work.
2. **Serial equivalence** still holds: PE and sequential produce the same state root on non-conflicting, mixed, and full-hub fixtures.
3. On a **mixed hub workload** (~25% txs touch a shared account), PE records a **non-zero rollback / conflict rate** (~0.23) while roots still match sequential.
4. On a **full hub** workload (all txs debit one account), multi-worker PE rolls back nearly every speculative attempt (`conflict_rate ≈ 0.98`) and is slower than sequential — correct but not a capacity win.
5. **Implication for Track 4:** full Block-STM is **not** justified by this simple-transfer matrix; only re-open if a heavier non-conflicting contract matrix shows capacity gains fork+overlay cannot deliver.

Falsifiers:

- Disjoint multi-worker simple transfers consistently `speedup ≥ 1.5×` on this machine class → re-check overhead assumptions.
- Any matrix cell with root mismatch → freeze-severity PE bug.
- Mixed fixture with zero rollbacks at workers ≥ 2 → harness no longer stresses shared keys.

---

## Harness commands

Prereq: Go 1.25+ from monorepo root. No live RPC required for PE load tests.

### Lab baseline (S0)

```bash
go test ./... -count=1
go test ./devnet/ -count=1
```

### PE load + matrix (S1–S2)

```bash
# Full load package (includes legacy non-conflict + mixed + matrix)
go test ./tests/load/ -count=1 -timeout 120s -v

# S2 matrix only (scenario × workers rows)
go test ./tests/load/ -count=1 -timeout 120s -run TestLoad_PE_Matrix -v

# PE unit path (equivalence + metrics)
go test ./core/vm/ -count=1 -timeout 60s -run 'Parallel' -v
```

Optional RPC stats after a local node has run PE blocks or admitted txs:

```bash
curl -s -X POST http://127.0.0.1:8545 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"dew_getExecutionStats","params":[]}'

curl -s -X POST http://127.0.0.1:8545 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"dew_getMempoolStats","params":[]}'
```

See [dew_getExecutionStats](../api/dew-extensions.md#dew_getexecutionstats) · [dew_getMempoolStats](../api/dew-extensions.md#dew_getmempoolstats) (S3).

### What to capture

| Field | Source |
| :--- | :--- |
| `seq_ms` / `par_ms` / `speedup` | matrix `ROW` log lines |
| `rollbacks`, `conflict_rate`, `spec_ok` | `Stats()` on PE path |
| `root_ok` | hard gate (test fails if false) |
| workers / scenario | log columns |

---

## Baseline run (S1)

| | |
| :--- | :--- |
| **Date** | 2026-07-13 |
| **Host** | macOS (local developer machine) |
| **Command** | `go test ./tests/load/ -v` and `go test ./core/vm/ -run 'Parallel' -v` |
| **Result** | all PASS |

### Early load samples

| Case | n | Workers | Sequential | Parallel | Speedup | Rollbacks | Conflict rate |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Non-conflicting transfers | 128 | 8 | 3.03 ms | 5.28 ms | **0.57×** | 0 | 0.000 |
| Mixed hub conflicts | 64 | default | — | — | — | **15** | **0.234** |

---

## PE matrix (S2)

| | |
| :--- | :--- |
| **Date** | 2026-07-13 |
| **Host** | macOS, `GOMAXPROCS=8` |
| **Command** | `go test ./tests/load/ -count=1 -run TestLoad_PE_Matrix -v` |
| **Fixture** | n=64 simple value transfers; gas price 0 (no coinbase write noise) |
| **Result** | all PASS · every cell `root_ok=true` |

Scenarios:

| Scenario | Conflict structure |
| :--- | :--- |
| `disjoint` | No shared accounts |
| `mixed_1in4` | Every 4th tx debits a shared hub (~25%) |
| `hub_all` | Every tx debits the same hub |

### Matrix table (simple transfers)

| Scenario | Workers | seq_ms | par_ms | Speedup | Rollbacks | Conflict rate | Spec OK |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| disjoint | 1 | 1.275 | 0.771 | 1.65 | 0 | 0.000 | 64 |
| disjoint | 2 | 1.275 | 1.937 | 0.66 | 0 | 0.000 | 64 |
| disjoint | 4 | 1.275 | 1.646 | 0.77 | 0 | 0.000 | 64 |
| disjoint | 8 | 1.275 | 2.096 | 0.61 | 0 | 0.000 | 64 |
| mixed_1in4 | 1 | 1.329 | 0.783 | 1.70 | 0 | 0.000 | 64 |
| mixed_1in4 | 2 | 1.329 | 1.996 | 0.67 | 15 | **0.234** | 49 |
| mixed_1in4 | 4 | 1.329 | 1.992 | 0.67 | 15 | **0.234** | 49 |
| mixed_1in4 | 8 | 1.329 | 2.520 | 0.53 | 15 | **0.234** | 49 |
| hub_all | 1 | 1.034 | 1.000 | 1.03 | 0 | 0.000 | 64 |
| hub_all | 2 | 1.034 | 2.549 | 0.41 | 63 | **0.984** | 1 |
| hub_all | 4 | 1.034 | 2.181 | 0.47 | 63 | **0.984** | 1 |
| hub_all | 8 | 1.034 | 2.731 | 0.38 | 63 | **0.984** | 1 |

Notes:

- **workers=1** PE path can beat `ApplySequential` wall-clock on this host (different code path / less instrumentation); multi-worker is the interesting PE case.
- Multi-worker **speedup &lt; 1** on all simple-transfer scenarios (supports H1.1).
- Conflict rate tracks structure: 0 → ~0.23 → ~0.98 (supports H1.3–4).
- **root_ok=true** for every cell (supports H1.2).

### Block-STM follow-up (S2 optional)

**Not filed for implementation.** Simple-transfer matrix does not show multi-worker PE capacity gains; residuals remain in [agents/debt.md](../../agents/debt.md) (PE fork+overlay). Revisit only with heavier non-conflicting contract load or production PE metrics that contradict H1.

### Reading vs H1

| Claim | S2 outcome |
| :--- | :--- |
| Multi-worker simple PE often slower | Supported (speedup 0.38–0.77 for workers ≥ 2) |
| Roots always match sequential | Supported (12/12 cells) |
| Mixed conflicts → non-zero rollbacks | Supported (~0.234 at workers ≥ 2) |
| Hub-all → near-total rollbacks | Supported (~0.984) |
| Block-STM now | **No** — measure first held |

**Still open (out of S2):** multiproc BFT commit latency, SMT tip growth, heavier contract PE matrix.

---

## Next (after S3)

- **S4** — staking precompile edges (`0x102`) in lab
- Optional: heavier PE fixtures only if product load needs it
