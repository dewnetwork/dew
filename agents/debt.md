# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

Phase mapping: items owned by an active phase live in [docs/development/phases.md](../docs/development/phases.md); this file tracks **residuals** and cross-cutting debt.

**Phase C (C1–C6) complete** — freeze tag `public-testnet-v1`. See [public-testnet.md](../docs/development/public-testnet.md).

**Phase D** — D1 explorer + D2 faucet done → D3 scale (Path A / C4 / audit) when needed. Design spec: [d3-scale.md](../docs/development/d3-scale.md). Acceptance in [phases.md](../docs/development/phases.md).

## C1 residuals

- [ ] Full fee auction / priority ordering for multi-tx block builders (admission-only today; auto-mine path includes immediately)
- [ ] Pending nonce-gap queue without immediate execute (dev node still auto-mines on admit)

## C4 residuals

Owned by D3c — see [d3-scale.md](../docs/development/d3-scale.md#d3c--staking-residuals-c4).

- [ ] Enforce unbonding period (time/height) before stake withdrawal
- [ ] Full double-sign evidence verification (not only non-zero hash placeholder)
- [ ] Nested CALL caller for bond (currently top-level tx sender)
- [ ] Wire ActiveSet into live Dew-BFT set rotation each epoch
- [ ] Delegation / commission

## C5 residuals

Owned by D3 — see [d3-scale.md](../docs/development/d3-scale.md) (D3a / D3b).

- [x] Multi-process binary packaging (systemd/docker compose samples) — `deploy/` (Dockerfile; `docker-compose.soak.yml` devnet/`multi`; `docker-compose.yml` public path B + nginx; systemd units); `dew run --p2p.*` for encrypted mesh packaging
- [ ] **D3a** Multi-process Dew-BFT shared block production (compose `multi` still auto-mines per process; use `dew devnet` for in-process BFT)
- [ ] **D3b** Persistent peer store / auto-redial after restart (manual RedialMesh in chaos test today; `dew run` has dial retry only at start)

## C6 residuals (ops, not freeze blockers)

- [x] Path B public surface live (July 2026) — `https://rpc-dew.fadosoft.com`, `https://faucet-dew.fadosoft.com`, `https://explorer-dew.fadosoft.com`; bootnodes n/a for path B
- [ ] Publish real public bootnode hostnames / multiaddrs when Path A multi-host launches (process documented; values not in-repo)
- [x] Production faucet service (rate limits, captcha) outside monorepo core — `faucet/` + `cmd/dewfaucet` (D2); see [docs/development/faucet.md](../docs/development/faucet.md)
- [ ] Longer continuous fuzz in CI (`-fuzztime` schedules)

## Deferred / mainnet

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`). Not required for first public testnet.
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md` and security principles stage table).
- [ ] Tokenomics issuance / inflation numbers still draft (not part of wire freeze).
