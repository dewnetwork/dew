# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`).
- [ ] DewTx has no mempool admission limits / fee auction yet — required before public testnet spam exposure.
- [ ] Cleartext P2P transport is dev-only — need encrypted transport before public networks.
- [ ] Staking precompile `0x102` is a revert stub — implement module before product UX.
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md`).
- [ ] Flat state root is provisional sorted-leaf commitment — replace with SMT before testnet freeze.
