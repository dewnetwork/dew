---
title: Research lab harness
description: Track R hypotheses, PE matrix, multiproc BFT latency, and recorded baselines.
category: ops
order: 55
status: stable
---

# Research lab harness

Lab note for **Track R** (S0–S2 PE, multiproc BFT, optional state). Written hypotheses, reproducible commands, and recorded baselines. Not marketing; wall-clock numbers vary by machine.

**Related:** [Parallel execution](../execution/parallel-execution.md) · [Phases — Track R](../build/phases.md#track-r--research-lab) · [Plan S0–S6](../build/phases.md#recommended-sequence--research-lab--precompile-slots) (done) · packages `tests/load/`, `core/vm`, `devnet/`.

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

### Multiproc BFT + chaos (Track R)

```bash
# Short commit-latency lab (CI-safe; logs BFT_ROW)
go test ./devnet/ -count=1 -timeout 180s -run TestMultiProcessBFT_CommitLatencyLab -v

# Shared tip correctness + transfer (CI multiproc smoke)
go test ./devnet/ -count=1 -timeout 180s -run TestMultiProcessBFT_SharedChain -v

# Chaos restart / re-dial / sync (logs BFT_CHAOS_ROW)
go test ./devnet/ -count=1 -timeout 60s -run TestPrivateNet_ChaosRestartAndSync -v

# Optional heavy empty soak ≥150 heights (logs BFT_ROW scenario=long_empty)
DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -count=1 -timeout 10m -run TestMultiProcessBFT_LongEmpty -v
```

### State / SMT growth (Track R H3)

```bash
# Flat hot path vs IntermediateRoot / Commit as durable leaves grow (logs STATE_ROW)
go test ./tests/load/ -count=1 -timeout 120s -run TestLoad_State_SMT_GrowthMatrix -v
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

**Still open after S2:** heavier contract PE matrix (only if product load needs it). Multiproc BFT: see [H2](#hypothesis-h2--multiproc-bft-pace-and-recovery) below. SMT tip growth remains optional.

---

## Hypothesis H2 — multiproc BFT pace and recovery

**H2 — empty-chain multiproc commit wall-clock and chaos recovery**

On the current multiproc Dew-BFT path (`devnet.StartMultiProcessBFT`, 3 validators + 1 full node, encrypted P2P default):

1. With a **lab** `MinBlockInterval` of 50 ms, mean wall-clock **per height** to a uniform tip (3 validators + full) is **on the order of 100–200 ms**, not the 50 ms floor — vote/gossip/catch-up dominate pure interval.
2. Production default interval (1 s) remains the right ops pace for empty blocks; lab intervals are for CI/research only (`--bft.min-block-interval`).
3. After **host kill + redial**, peer rejoin is **sub-second** on loopback; range **sync of a few heights** is also sub-second on this harness.
4. Uniform tip at height \(N\) across validators and full node remains a hard gate (no split heads in the lab window).

Falsifiers:

- Lab mean ms/height consistently **&lt; min_interval** with uniform tip → timing bug or wrong start clock.
- Chaos redial/sync regularly **&gt; 3 s** on loopback → mesh/sync regression.
- Any validator/full tip hash mismatch after `waitUniformTip` success → freeze-severity consensus bug.

### Baseline run (H2)

| | |
| :--- | :--- |
| **Date** | 2026-07-14 |
| **Host** | macOS (local developer machine) |
| **Commands** | `go test ./devnet/ -run TestMultiProcessBFT_CommitLatencyLab -v` · `TestPrivateNet_ChaosRestartAndSync -v` |
| **Result** | all PASS |

#### Commit latency (`BFT_ROW`)

| Scenario | Tip | Elapsed | Mean ms/height | Heights/s | Min interval |
| :--- | ---: | ---: | ---: | ---: | ---: |
| `commit_latency_lab` | 12 | 1724 ms | **143.7** | 6.96 | 50 ms |

Notes:

- Mean ≫ 50 ms supports H2.1 (interval is a floor, not end-to-end finality).
- Heavy soak `long_empty` (150 heights, 200 ms interval) is optional: `DEW_HEAVY_INTEGRATION=1` · same `BFT_ROW` log format.

#### Chaos recovery (`BFT_CHAOS_ROW`)

| Metric | Value |
| :--- | ---: |
| Redial + peer wait | **1.9 ms** |
| Sync to height 3 | **10.7 ms** |

Supports H2.3 on loopback. Operator Path A / WAN will be slower; this is a harness floor, not a public SLA.

### Reading vs H2

| Claim | Outcome |
| :--- | :--- |
| Lab mean ms/height ≫ min_interval | Supported (~144 ms vs 50 ms) |
| Chaos recovery sub-second loopback | Supported (~13 ms total redial+sync) |
| Uniform tip hard gate | Supported (test PASS) |

---

## Hypothesis H3 — SMT commit cost vs tip state size

**H3 — flat hot path stays cheap; SMT IntermediateRoot tracks total durable leaves**

On the current state model (`core/state`: flat KV hot path, commit-time SMT in `smt.go`):

1. **Flat writes** (balance/account dirties) stay **cheap** and roughly linear in the number of new leaves in a batch (sub-millisecond to low-ms for thousands of accounts in-process).
2. **`IntermediateRoot` / SMT** cost grows with **total durable state leaves**, not only the block’s dirty set — because `collectLeaves` walks durable state via `IteratePrefix` and rebuilds the sparse tree every time.
3. A **single dirty account** on a large tip has **similar SMT wall-clock** to a clean recompute (not O(dirty-only)).
4. **`Commit`** (flush dirty KV + IntermediateRoot) is dominated by flush + full SMT; it also grows with tip size.

Falsifiers:

- Clean SMT time stays flat while leaves grow 32→2048 → measurement bug or incomplete walk.
- One-dirty SMT consistently ≪ clean SMT by orders of magnitude at large N → dirty-only optimization landed undocumented.
- Flat write batch for 2k accounts regularly multi-second → hot-path regression.

### Baseline run (H3)

| | |
| :--- | :--- |
| **Date** | 2026-07-14 |
| **Host** | macOS (local developer machine) |
| **Command** | `go test ./tests/load/ -count=1 -run TestLoad_State_SMT_GrowthMatrix -v` |
| **Fixture** | cumulative simple account leaves (balance only); Pebble temp DB |
| **Result** | all PASS |

#### Growth table (`STATE_ROW`)

| Leaves | Δ | flat_ms | commit_ms | clean_smt_ms | one_dirty_smt_ms |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 32 | 32 | 0.38 | 142 | **4.7** | 4.4 |
| 128 | 96 | 0.08 | 468 | **20.2** | 17.5 |
| 512 | 384 | 0.44 | 1740 | **64.6** | 64.4 |
| 2048 | 1536 | 1.00 | 6047 | **253.6** | 251.6 |

Notes:

- clean_smt ≈ one_dirty_smt at each size → supports H3.2–3 (full rescan, not dirty-only).
- clean_smt scales ~linearly with leaves (~4.7→253 ≈ ×54 for ×64 leaves).
- flat_ms stays ~1 ms at 1536 new accounts → supports H3.1.
- Implication for ops: long-lived tips pay **growing commit-time SMT** even for tiny blocks; incremental/dirty SMT would be a future core optimization (not required for public-testnet-v1 freeze).

### Reading vs H3

| Claim | Outcome |
| :--- | :--- |
| Flat hot path cheap | Supported |
| SMT grows with tip leaves | Supported (~linear in matrix) |
| One dirty ≈ full clean SMT | Supported |
| Need Block-STM for state | **No** — separate from PE; optional dirty SMT residual |

---

## Staking lab (S5)

In-process path with staking **on** (not public Path B default):

```bash
go test ./node/ -count=1 -run TestStakingLab_Scenario -v
```

Scenario: bond → ActiveSet rank → epoch rotation → unbond/withdraw → optional jail. Ops notes: [private-testnet — Staking lab](./private-testnet.md#staking-lab-s5). Actor rules: [precompiles 0x102](../execution/precompiles.md#0102--staking-entrypoint-phase-c4).

## Next (Track R lab surface largely filled)

Track R acceptance (PE + BFT + state) is **done** for the research harness. Optional follow-ups:

- Track **4** residual: PE Block-STM only if product load contradicts H1; optional **dirty/incremental SMT** if tip growth becomes ops-painful (H3); log-index O(range) **done** 2026-07-14
- Deferred: heavier PE fixtures, Path A, live `0x101`, delegation / slash %
