# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

Phase mapping: items owned by an active phase live in [docs/development/phases.md](../docs/development/phases.md); this file tracks **residuals** and cross-cutting debt.

## Phase C (in scope — see phases.md)

- [x] DewTx / EVM mempool admission limits & fee checks — **C1** (done: `mempool/`)
- [x] Cleartext P2P transport is dev-only — encrypted transport **C2** (X25519 + AES-GCM)
- [x] Flat state root is provisional sorted-leaf commitment — replace with SMT **C3**
- [ ] Staking precompile `0x102` is a revert stub — implement module **C4**

## C1 residuals

- [ ] Full fee auction / priority ordering for multi-tx block builders (admission-only today; auto-mine path includes immediately)
- [ ] Pending nonce-gap queue without immediate execute (dev node still auto-mines on admit)

## Deferred / mainnet

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`). Not required for first public testnet.
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md` and security principles stage table).
