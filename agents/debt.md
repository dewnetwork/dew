# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

Phase mapping: items owned by an active phase live in [docs/build/phases.md](../docs/build/phases.md); this file tracks **residuals** and cross-cutting debt.

**Phase C (C1–C6) complete** — freeze tag `public-testnet-v1`. See [public-testnet.md](../docs/ops/public-testnet.md).

**Phase D foundation complete** — D1 explorer + D2 faucet; D3a + D3b + D3a residual; **durable chaindata**; product-v1. Open work is **tracks** (one row each): R research, 1 product, 2 protocol, 3 ops, 4 core, 5 mainnet — not forced mainnet. [roadmap — Tracks](../docs/build/roadmap.md#tracks) · [phases — Tracks](../docs/build/phases.md#tracks). Optional scale: D3c / D3d / D3e on demand — [d3-scale.md](../docs/scale/d3-scale.md), [durable-chaindata.md](../docs/ops/durable-chaindata.md).

**Product & scale upgrades** — full backlog in [docs/product/upgrades.md](../docs/product/upgrades.md). Track 1 product-v1 through v1.2 + live path B rebuild **done** (2026-07-13); **P1e indexer shipped** 2026-07-14; deferred: P1f / P3c / internal txs. Research-friendly default rows: Track **R** + Track **4**; Track **5** only before mainnet claims.

**Plan S0–S6 complete (Checkpoint C)** — [phases.md § Recommended sequence](../docs/build/phases.md#recommended-sequence--research-lab--precompile-slots). Precompile slots registry formalized: `vm.DewPrecompileSlots()` + [addresses.md](../docs/protocol/addresses.md#precompile-slots-evm-space) (`0x100` active / `0x101` reserved / `0x102` flagged); next free `0x103`.

**Track R lab surface complete** — H1 PE + H2 multiproc BFT + H3 SMT growth (2026-07-14, [research-lab.md](../docs/ops/research-lab.md)). **Track 4 lazy hydrate + log index + WS eth_subscribe done** 2026-07-14. **P1e dewindex sidecar done** 2026-07-14. **Filter API (gap Wave 3) done** 2026-07-15. **P1f ABI registry done** 2026-07-15 (`dewindex` contracts + explorer Contract tab; badge “ABI registered”, not solc match). Residuals: live `0x101` orderbook (needs design + hardfork), delegation, slash %, PE Block-STM, optional dirty/incremental SMT (H3), product P3c.

## Durable chaindata

- [x] Pebble backend (`db.PebbleDB`) behind existing `db.Database`
- [x] Persist headers/bodies/canonical/receipts/tx index + tip under `<datadir>/chaindata`
- [x] Atomic batch with flat state (`a`/`s`/`c`) on seal + `ImportCommittedBlock`
- [x] `node.Open` + restart recovery; genesis mismatch refuses start
- [x] `dew run --datadir` opens chaindata; compose volumes for Path B / multi
- [x] **Not** merging peers into chaindata — keep `<datadir>/peers.json` (D3b)
- [x] Pebble is the only DB backend; MemoryDB removed; `--datadir` defaults to `/var/lib/dew`
- [x] Lazy hydrate: Open tip-only; blocks/tx/receipts/logs on demand — Track 4 (2026-07-14)
- [x] Secondary log-index keys (`L|block|tx|log`) for O(range) `eth_getLogs` + Open backfill — Track 4 residual (2026-07-14)

## C1 residuals

- [x] Multi-tx block packing + fee auction over continuous nonce chains (`selectPendingTxsForBlockLocked`, `DefaultMaxTxsPerBlock=64`, auto-mine pack) — July 2026
- [x] Pending nonce-gap queue (`nonce >= account`; future nonces stay pending until gap filled) — July 2026
- [x] Multi-tx / gap for **DewTx** auto-mine path (`selectPendingDewTxsForBlockLocked`, `sealReadyDewFromPoolLocked`; body still empty under public-testnet-v1) — July 2026
- [x] Partial re-select when simulation fails mid-block (`simulateAndFilterLocked` + exclude/retry in `BuildBlockFromPool`) — July 2026

## C4 residuals

Owned by D3c — see [d3-scale.md](../docs/scale/d3-scale.md#d3c--staking-residuals-c4).

- [x] Enforce unbonding period (block timestamp) before stake withdrawal — unbond queue + `0x08` withdraw + `0x09` pending (July 2026)
- [x] Full double-sign evidence verification (dual-vote wire + VerifyDoubleSign) (July 2026)
- [x] Nested CALL bond credits immediate value-payer (Transfer hook) (July 2026)
- [x] Wire ActiveSet into live Dew-BFT set rotation each epoch (when staking on) (July 2026)
- [ ] On-chain slash burn percentages (economics still tentative) — **S4 deferred** with notes in tokenomics/slashing (2026-07-14)
- [x] Zero-value nested unbond/withdraw still use tx origin (EVM precompile has no call stack) — **documented fail-closed + tests** (S4, 2026-07-14); hardfork if contract self-unbond is required
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
- [x] GitHub Release Please + cross-platform Go binaries (`release-please.yml` / `release-binaries.yml`) — July 2026
- [x] GHCR multi-arch image push on release (`release-images.yml`: dew, faucet, explorer, guestbook) — July 2026
- [x] Compose path B / full stack default to GHCR pull (`image:` + `pull_policy`; `build:` kept for `--build`) — July 2026
- [x] `ldflags` version injection for `dew` / `dewcli` / `dewfaucet` / `dewindex` / `web3_clientVersion` (`version` package; release CI + Docker `VERSION` arg) — July 2026

## Deferred / mainnet

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`). Not required for first public testnet.
- [ ] Optional: dirty / incremental SMT commit — H3 shows IntermediateRoot walks full durable state; only if tip growth becomes ops-painful (not freeze-required).
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md` and security principles stage table).
- [ ] Tokenomics issuance / inflation numbers still draft (not part of wire freeze).
- [ ] Live `0x101` native swap / orderbook — **reserved only** after S6 (2026-07-14); needs separate design + hardfork doc under public-testnet-v1 before activation.

## Product surface (product-v1)

- [x] Upgrade backlog doc [docs/product/upgrades.md](../docs/product/upgrades.md) — all tracks 1–5
- [x] Explorer P1a ERC-20 metadata · P1b method + token transfer logs · P1c recent search
- [x] Faucet P2a success actions · P2b rate-limit copy
- [x] Guestbook P3a author / mine filter
- [x] P1d known-token balances · P2c wallet paste · P3b share `?author=` (product-v1.1)
- [x] P2d faucet funder `balanceWei` on `/info` + UI (product-v1.2)
- [x] Deploy rebuild of live path B surfaces (operator) — live verify 2026-07-13 via public HTTPS only; **no SSH** to path B host from this workflow (see [upgrades.md](../docs/product/upgrades.md) product-v1 DoD)
- [x] Optional (host console only, not SSH from monorepo agents): pin GHCR to a release that includes ldflags inject + recreate stack so live `web3_clientVersion` matches the tag — operator deploy 2026-07-14 (`v0.5.0`)
- [x] P1e indexer history / volume (`dewindex` + explorer) — 2026-07-14
- [x] P1f ABI/source registry (`dewindex` + explorer) — 2026-07-15
- [ ] P3c reactions (deferred)

## Docs

- [x] Reorg `docs/development/` → `build/` · `ops/` · `product/` · `scale/` (July 2026)
- [x] Public DX surface docs — try-public, Guestbook product, freeze/publish template, recipes C1 (July 2026)
- [x] Hardhat example parity with Foundry (`examples/hardhat/` + quickstart §3b) — July 2026
- [x] Multi-tx burst UI on Guestbook SPA (`Burst ×2`) + docs — July 2026
- [x] Landing `web/` sync to public-testnet-v1 — live Network section, A–D phases, try-public CTAs, `src/lib/network.ts` (July 2026)
- [x] Landing polish — mobile nav sheet, OG image (`public/og.png` + `og.svg`), dual-tab Builder terminal (July 2026)
- [ ] VitePress is pinned to **2.0.0-alpha.18** (Vite 8) so `pnpm audit --audit-level=high` is clean; mermaid via `vitepress-mermaid-renderer` (no VitePress 1 peer). Revisit when VitePress 2 stable ships — drop alpha pin and re-check mermaid + theme.
- [x] Promote protocol/execution pages from `status: draft` to `stable` after prose pass (aligned to public-testnet-v1) — July 2026
- [x] Promote overview, architecture category, consensus, networking, security, api, genesis, ops devnet/private-testnet to `stable` — July 2026
- [ ] `docs/economics/tokenomics.md` stays **draft** until issuance / inflation / reward-split numbers freeze for mainnet
