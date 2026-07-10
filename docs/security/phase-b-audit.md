---
title: Phase B Security Audit (internal)
description: Checklist and findings for Dew-PE, DewTx, and system precompiles before broader review.
category: security
order: 30
status: draft
---

# Phase B Security Audit (internal)

> Internal engineering audit for Phase B surfaces. Complements [Threat Model](./threat-model.md) and [Security Principles](./security-principles.md). Not a substitute for an external mainnet audit.

## Scope

| Component | Packages | Risk themes |
| :-------- | :------- | :---------- |
| Dew-PE | `core/vm/parallel.go`, `core/state` | Non-determinism → consensus split |
| DewTx | `core/types/dewtx.go`, `core/native` | Replay, fee bypass, access-list privilege |
| Precompiles | `core/vm/precompiles.go` | Stateful value theft, unexpected CALL semantics |
| RPC | `rpc` dew_* | Feature-flag bypass, malformed hex |

Out of scope here: external dependency CVE sweep, formal BFT proof, mainnet key ceremony.

## Checklist

### Determinism & PE

- [x] Parallel results match sequential roots/receipts on disjoint workload (`tests/load`, `core/vm`)
- [x] Mixed conflict workload re-executes and still matches sequential roots
- [x] `Finalise` only purges empties dirtied this tx (no global cache wipe divergence)
- [x] Rollback / conflict metrics exposed (`dew_getExecutionStats`)

### Domain separation & crypto

- [x] DewTx signing domain `DewTx:v1` ≠ EVM tx hash (`core/types` tests)
- [x] Recovered sender must equal declared `Sender` field
- [x] Wrong chain ID rejected at node boundary
- [x] EVM-typed prefix (`0x02` etc.) rejected by DewTx decoder

### Fail closed

- [x] Incomplete AccessList on credit module → failed result, no state mutation
- [x] Unknown payload module → fail closed
- [x] Native path disabled → `dew_sendRawTransaction` errors
- [x] Precompiles disabled → `0x100` does not forward value

### Fees & DoS

- [x] Flat fee constant / min fee documented; `Fee=0` still charges default
- [x] Fixed precompile gas (no unbounded native work)
- [x] Load tests establish throughput floor for native path

### Precompiles

- [x] `0x100` requires exactly 20-byte recipient
- [x] Value sits at precompile only if flag off (no silent forward)
- [x] `0x102` reserved stub reverts (no fake stake)

## Residual risks / debt

| Item | Severity | Mitigation / follow-up |
| :--- | :------- | :--------------------- |
| PE overlay is not full MVCC Block-STM | Medium | Serial-equivalent tests; bound workers; fallback re-exec |
| Cleartext P2P (dev) | High on public net | Keep private; encrypt later |
| No mempool fee auction for DewTx | Low (dev) | Add mempool limits before public testnet |
| Staking precompile stub | Info | Do not enable product UX until module lands |
| External audit of consensus + bridge | High before mainnet | See security principles stage table |

Record durable engineering debt in `agents/debt.md` when items become code tasks.

## Test commands

```bash
go test ./...
go test ./tests/load/ -count=1
go test ./tests/security/ -count=1
go test -bench=. -benchtime=3x ./core/vm/ ./core/native/
```

## Sign-off (Phase B4)

| Role | Status |
| :--- | :----- |
| Engineering self-audit | Complete for listed checklist items |
| External audit | **Not done** — required before mainnet |
