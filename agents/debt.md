# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

Phase mapping: items owned by an active phase live in [docs/development/phases.md](../docs/development/phases.md); this file tracks **residuals** and cross-cutting debt.

## Phase C (in scope — see phases.md)

- [ ] DewTx / EVM mempool admission limits & fee checks — **C1**
- [ ] Cleartext P2P transport is dev-only — encrypted transport **C2**
- [ ] Flat state root is provisional sorted-leaf commitment — replace with SMT **C3**
- [ ] Staking precompile `0x102` is a revert stub — implement module **C4**

## Deferred / mainnet

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`). Not required for first public testnet.
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md` and security principles stage table).
