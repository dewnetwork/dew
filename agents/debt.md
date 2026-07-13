# Technical Debt

Below is a list of technical debt, refactoring tasks, or optimization items in the project.

Phase mapping: items owned by an active phase live in [docs/build/phases.md](../docs/build/phases.md); this file tracks **residuals** and cross-cutting debt.

**Phase C (C1–C6) complete** — freeze tag `public-testnet-v1`. See [public-testnet.md](../docs/ops/public-testnet.md).

**Phase D** — D1 explorer + D2 faucet done; D3a + D3b + D3a residual done; **durable chaindata done** → D3d Path A / D3c / D3e when needed. Design: [durable-chaindata.md](../docs/ops/durable-chaindata.md). D3 scale: [d3-scale.md](../docs/scale/d3-scale.md). Acceptance in [phases.md](../docs/build/phases.md).

**Product & scale upgrades** — full backlog in [docs/product/upgrades.md](../docs/product/upgrades.md). Track 1 (product) first: product-v1 P1a–c / P2a–b / P3a shipped; remaining P1d–f / P2c–d / P3b–c planned or deferred.

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
- [x] Multi-tx / gap for **DewTx** auto-mine path (`selectPendingDewTxsForBlockLocked`, `sealReadyDewFromPoolLocked`; body still empty under public-testnet-v1) — July 2026
- [x] Partial re-select when simulation fails mid-block (`simulateAndFilterLocked` + exclude/retry in `BuildBlockFromPool`) — July 2026

## C4 residuals

Owned by D3c — see [d3-scale.md](../docs/scale/d3-scale.md#d3c--staking-residuals-c4).

- [x] Enforce unbonding period (block timestamp) before stake withdrawal — unbond queue + `0x08` withdraw + `0x09` pending (July 2026)
- [x] Full double-sign evidence verification (dual-vote wire + VerifyDoubleSign) (July 2026)
- [x] Nested CALL bond credits immediate value-payer (Transfer hook) (July 2026)
- [x] Wire ActiveSet into live Dew-BFT set rotation each epoch (when staking on) (July 2026)
- [ ] On-chain slash burn percentages (economics still tentative)
- [ ] Zero-value nested unbond/withdraw still use tx origin (EVM precompile has no call stack)
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
- [ ] Optional: compose samples default to `image: ghcr.io/...:tag` instead of `build:` for path B
- [ ] Optional: `ldflags` version injection for `dew` / `dewcli` / `web3_clientVersion`

## Deferred / mainnet

- [ ] PE uses fork+overlay, not full multi-version Block-STM — higher memory and re-exec cost under heavy conflicts (`core/vm/parallel.go`). Not required for first public testnet.
- [ ] External security audit of consensus + VM bridge + crypto — required before mainnet (see `docs/security/phase-b-audit.md` and security principles stage table).
- [ ] Tokenomics issuance / inflation numbers still draft (not part of wire freeze).

## Product surface (product-v1)

- [x] Upgrade backlog doc [docs/product/upgrades.md](../docs/product/upgrades.md) — all tracks 1–5
- [x] Explorer P1a ERC-20 metadata · P1b method + token transfer logs · P1c recent search
- [x] Faucet P2a success actions · P2b rate-limit copy
- [x] Guestbook P3a author / mine filter
- [x] P1d known-token balances · P2c wallet paste · P3b share `?author=` (product-v1.1)
- [x] P2d faucet funder `balanceWei` on `/info` + UI (product-v1.2)
- [ ] P1e indexer history · P1f verified source · P3c reactions (deferred)

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
