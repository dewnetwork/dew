# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

Phase mapping: items owned by an active phase live in [docs/build/phases.md](../docs/build/phases.md); this file tracks **residuals** and cross-cutting debt.

**Phase C (C1–C6) complete** — freeze tag `public-testnet-v1`. See [public-testnet.md](../docs/ops/public-testnet.md).

**Phase D** — D1 explorer + D2 faucet done; D3a + D3b + D3a residual done; **durable chaindata done** → D3d Path A / D3c / D3e when needed. Design: [durable-chaindata.md](../docs/ops/durable-chaindata.md). D3 scale: [d3-scale.md](../docs/scale/d3-scale.md). Acceptance in [phases.md](../docs/build/phases.md).

## Durable chaindata

- [x] Pebble backend (`db.PebbleDB`) behind existing `db.Database`
- [x] Persist headers/bodies/canonical/receipts/tx index + tip under `<datadir>/chaindata`
- [x] Atomic batch with flat state (`a`/`s`/`c`) on seal + `ImportCommittedBlock`
- [x] `node.Open` + restart recovery; genesis mismatch refuses start
- [x] `dew run --datadir` opens chaindata; compose volumes for Path B / multi
- [x] **Not** merging peers into chaindata — keep `<datadir>/peers.json` (D3b)
- [x] Pebble is the only DB backend; MemoryDB removed; `--datadir` defaults to `/var/lib/dew`
- [ ] Optional: lazy hydrate / log index keys when tip is very large

## C1 residuals

- [x] Multi-tx block packing + fee auction over continuous nonce chains (`selectPendingTxsForBlockLocked`, `DefaultMaxTxsPerBlock=64`, auto-mine pack) — July 2026
- [x] Pending nonce-gap queue (`nonce >= account`; future nonces stay pending until gap filled) — July 2026
- [ ] Optional: multi-tx / gap for **DewTx** auto-mine path (EVM path done; Dew still per-tx when auto-mine)
- [ ] Optional: partial re-select when simulation fails mid-block (today drops all pending selection on sim error)

## C4 residuals

Owned by D3c — see [d3-scale.md](../docs/scale/d3-scale.md#d3c--staking-residuals-c4).

- [ ] Enforce unbonding period (time/height) before stake withdrawal
- [ ] Full double-sign evidence verification (not only non-zero hash placeholder)
- [ ] Nested CALL caller for bond (currently top-level tx sender)
- [ ] Wire ActiveSet into live Dew-BFT set rotation each epoch
- [ ] Delegation / commission

## C5 residuals

Owned by D3 — see [d3-scale.md](../docs/scale/d3-scale.md) (D3a / D3b).

- [x] Multi-process binary packaging (systemd/docker compose samples) — `deploy/` (Dockerfile; `docker-compose.soak.yml` devnet/`multi`; `docker-compose.yml` public path B + nginx; systemd units); `dew run --p2p.*` for encrypted mesh packaging
- [x] **D3a** Multi-process Dew-BFT shared block production (`--validator`, `node.Stack`, compose `multi` + `node-rpc`; `go test ./devnet/ -run MultiProcessBFT_SharedChain`)
- [x] **D3a** Multi-process ERC-20 integration (`DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run MultiProcessBFT_ERC20`) — pre-admit before BFT; validator catch-up + outbound write queue
- [x] **D3a residual:** multiproc empty-block pace (default `MinBlockInterval` 1s, `--bft.min-block-interval`) + bulk outbound drop under queue pressure; soak `DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run MultiProcessBFT_LongEmpty`
- [x] **D3b** Persistent peer store / auto-redial (`peers.json`, Host maintain loop, `--datadir`; compose multi volumes) — chaos `RedialHost` still used for ephemeral-port restart tests

## C6 residuals (ops, not freeze blockers)

- [x] Path B public surface live (July 2026) — `https://rpc-dew.fadosoft.com`, `https://faucet-dew.fadosoft.com`, `https://explorer-dew.fadosoft.com`; bootnodes n/a for path B
- [ ] Publish real public bootnode hostnames / multiaddrs when Path A multi-host launches (process documented; values not in-repo)
- [x] Production faucet service (rate limits, captcha) outside monorepo core — `faucet/` + `cmd/dewfaucet` (D2); see [docs/product/faucet.md](../docs/product/faucet.md)
- [x] Longer continuous fuzz in CI (`-fuzztime` schedules) — [`.github/workflows/security.yml`](../.github/workflows/security.yml) (PR short; weekly/manual 2m+1m)

## Deferred / mainnet

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`). Not required for first public testnet.
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md` and security principles stage table).
- [ ] Tokenomics issuance / inflation numbers still draft (not part of wire freeze).

## Docs

- [x] Reorg `docs/development/` → `build/` · `ops/` · `product/` · `scale/` (July 2026)
- [x] Public DX surface docs — try-public, Guestbook product, freeze/publish template, recipes C1 (July 2026)
- [ ] Hardhat example parity with Foundry (`examples/hardhat/` + quickstart section)
- [ ] Optional multi-tx burst UI demo (recipes already cover forge batch)
- [ ] VitePress is pinned to **2.0.0-alpha.18** (Vite 8) so `pnpm audit --audit-level=high` is clean; mermaid via `vitepress-mermaid-renderer` (no VitePress 1 peer). Revisit when VitePress 2 stable ships — drop alpha pin and re-check mermaid + theme.
- [ ] Promote remaining protocol/execution pages from `status: draft` to `stable` after prose pass
