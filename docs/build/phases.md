---
title: Implementation Phases
description: Acceptance criteria for Phases A–D and open tracks (research, product, protocol, ops, core, mainnet).
category: build
order: 30
status: stable
---

# Implementation Phases

Each phase should leave the **monorepo buildable and testable** (Go packages and any Node scripts that phase introduces). Prefer small PRs per phase.

### Phases A–D (foundation)

| Band | Status | Theme |
| :--- | :----- | :---- |
| **A1–A7** | Done | ETH-compatible L1 + local multi-validator devnet |
| **B1–B4** | Done | Dew-PE, DewTx, precompiles, load/security baselining |
| **C1–C6** | **Done** (C6 = public-testnet-v1 freeze) | Mempool, encrypted P2P, SMT, staking, private → public freeze |
| **D1–D2** | **Done** | Product surface — explorer + faucet |
| **D3a / D3b / chaindata** | **Done** | Multiproc BFT, peer redial, Pebble durable tip |
| **Product-v1** | **Done** (P1a–d / P2a–d / P3a–b · live path B 2026-07-13) | Post-MVP explorer/faucet/Guestbook — [upgrades](../product/upgrades.md) |

### Tracks (open work — one row each)

Not a Phase E. Pick **one track row** (or the ordered plan). Same freeze (`public-testnet-v1` / chain **2205**) unless a hardfork is documented.

| Track | Status | Theme | Detail |
| :--- | :----- | :---- | :--- |
| **R — Research lab** | Partial | H1 PE + H2 multiproc BFT done; state optional | [Track R](#track-r--research-lab) · [research-lab](../ops/research-lab.md) |
| **1 — Product** | Partial | v1–v1.2 done; P1e–f / P3c deferred | [Track 1](#track-1--product-surface) |
| **2 — Protocol (D3c)** | Partial | MVP + S4 actor docs; slash % / delegation open | [Track 2](#track-2--protocol--d3c-staking) |
| **3 — Ops (D3d)** | Optional | Path A multi-host public | [Track 3](#track-3--ops--d3d-path-a) |
| **4 — Core node** | Partial | Mempool telemetry + lazy hydrate done; PE upgrade open | [Track 4](#track-4--core-node) |
| **5 — Mainnet (D3e)** | Optional | Audit pack only before production claims | [Track 5](#track-5--mainnet-gate-d3e) |
| **Plan S0–S6** | **Done** (Checkpoint C 2026-07-14) | Ordered path research → Precompile slots | [Recommended sequence](#recommended-sequence--research-lab--precompile-slots) |

High-level order: [Roadmap](./roadmap.md) · track map: [upgrades](../product/upgrades.md).

**Ops note:** **public-testnet-v1 is live** on path B (July 2026) — see [Public testnet freeze](../ops/public-testnet.md#live-network-path-b). Path B is an optional public lab surface; private soak and launch checklist A–B remain the runbook for new hosts. Phases and tracks assume a live or local RPC (`chainId` **2205`) and do **not** re-open the C6 wire freeze.

**Default for research (no real mainnet users):** Track **R** + Track **4** (and Track **2** for staking lab). Defer Track **3** and Track **5**.

---

## A1 — Cryptography & wallet CLI

**Goals:** Keys, addresses, sign/verify, encrypted keystore.

**Acceptance:**

- [x] Generate secp256k1 keypair
- [x] Derive Ethereum-compatible address
- [x] Sign and verify a message / digest
- [x] `dewcli` create wallet + list address

**Packages:** `crypto/`, `crypto/wallet/`, `cmd/dewcli`

**Notes:** Keys use go-ethereum secp256k1 + Keccak; wallets stored encrypted (Web3 Secret Storage) under `~/.dew/keystore` (override with `--keystore` or `DEW_KEYSTORE`).

---

## A2 — Types & database

**Goals:** Persist accounts, blocks, and txs.

**Acceptance:**

- [x] `Account`, `Header`, `Block`, EVM tx types
- [x] KV put/get round-trip
- [x] Apply genesis `alloc` to state
- [x] Canonical header hash stable in tests

**Packages:** `core/types`, `core/state`, `db`, `config`

**Notes:** Header hash = `Keccak-256(RLP(header fields))`. At A2 the state root was a provisional sorted-leaf commitment; **C3 replaced it with SMT** (`header.StateRoot` = SMT root). Sample `genesis.json` at repo root; load via `config.LoadGenesisFile`.

---

## A3 — EVM execution

**Goals:** Run Solidity bytecode sequentially.

**Acceptance:**

- [x] StateDB bridge implements required interface
- [x] Deploy ERC-20 in-process
- [x] Transfer updates balances; events in receipt
- [x] Failed tx reverts state correctly

**Packages:** `core/vm`, `core/state`

**Notes:** Cancun-era rules via go-ethereum `core/vm`; Dew `StateDB` journal + access list + EIP-6780; `Executor.ApplyMessage` for create/call. ERC-20 fixture: solc 0.8.24 optimized `Token` in `core/vm/token_bytecode.go`.

---

## A4 — JSON-RPC

**Goals:** External tools talk to the node.

**Acceptance:**

- [x] HTTP RPC on 8545
- [x] Methods in [JSON-RPC required list](../api/json-rpc.md)
- [x] MetaMask shows balance _(dev: connect http://127.0.0.1:8545, chainId 2205)_
- [x] Foundry/Hardhat deploy succeeds against local node _(dev auto-mine packs ready txs)_
- [x] Optional: Node smoke script under monorepo `scripts/` hits `eth_chainId`

**Packages:** `node/`, `rpc/`, `cmd/dew`, `scripts/smoke-rpc.mjs`

**Notes:** Dev mode seals one block per `eth_sendRawTransaction` (BFT in A5). Start: `go run ./cmd/dew run --genesis genesis.json --http.port 8545`.

---

## A5 — Dew-BFT (local)

**Goals:** Propose and commit blocks with votes.

**Acceptance:**

- [x] Round state machine advances height
- [x] Single validator auto-commits (dev mode)
- [x] Three local validators reach commit with \(>2/3\) votes
- [x] Invalid root → prevote nil

**Packages:** `consensus/`

**Notes:** Tendermint-style steps (NewRound → Propose → Prevote → Precommit → Commit). Quorum is strict `>2/3` voting power. Stake-weighted round-robin proposer selection. In-process `LocalCluster` exercises multi-validator without P2P (A6). Invalid state root fails `ProposalValidator` → nil prevote → no commit.

---

## A6 — P2P

**Goals:** Multi-machine capable networking.

**Acceptance:**

- [x] Handshake + peer store
- [x] Tx and block gossip
- [x] Sync from height 0 behind peer
- [x] Consensus messages delivered among validators

**Packages:** `p2p/`

**Notes:** TCP framing `uint32be length || uint8 type || payload` with RLP payloads. Signed handshake (chain ID, height, node key). Inventory/GetData gossip for txs and blocks; `GetBlocks` range sync. Consensus channel `0x10–0x12` floods proposals/votes. A6 shipped cleartext for private devnets; **C2 default is encrypted** (`Encrypt=true`).

---

## A7 — Devnet

**Goals:** End-to-end demo.

**Acceptance:**

- [x] `genesis.json` + init instructions
- [x] 3 validators, 1 RPC node
- [x] Deploy and use ERC-20 over RPC
- [x] Document chain ID, ports, faucet

**Packages:** `devnet/`, `cmd/dew` (`init`, `devnet`), `scripts/devnet-erc20.mjs`

**Notes:** `dew init` writes genesis with 3 Anvil-compatible validators + faucet alloc. `dew devnet` starts in-process LocalCluster BFT, loopback P2P mesh, and JSON-RPC (default `:8545`). ERC-20 fixture over RPC covered by `go test ./devnet/`. Operator guide: [Local Devnet](../ops/devnet.md).

---

## B1 — Parallel execution

**Acceptance:**

- [x] Same fixtures as sequential (roots + receipts)
- [x] Speedup on non-conflicting workloads
- [x] Metrics for rollback rate

**Packages:** `core/vm` (`parallel.go`), `core/state` (Copy / access tracking / overlay)

**Notes:** Optimistic Block-STM style: speculative execute on `StateDB.Copy()`, validate read/write sets in index order, re-exec on conflict, `ApplyOverlay` for non-conflicting. Metrics via `ExecutionStats` / `dew_getExecutionStats`. Free-gas fixtures avoid coinbase tip conflicts for speedup demos.

---

## B2 — DewTx

**Acceptance:**

- [x] Codec + signature domain frozen
- [x] `dew_sendRawTransaction` works
- [x] Fail closed on incomplete access lists

**Packages:** `core/types` (`dewtx.go`), `core/native`, `node`, `rpc`, `params`

**Notes:** Wire `0xdf \|\| RLP(signed)`; signing hash = `Keccak256(Keccak256("DewTx:v1") \|\| RLP(unsigned))`. Flat fee `params.DefaultDewTxFeeWei`. Native executor fail-closes on undeclared credit targets. Feature flag `Node.SetNativeEnabled`.

---

## B3 — Native modules / precompiles

**Acceptance:**

- [x] At least one useful precompile behind feature flag
- [x] Gas/fee schedule documented
- [x] EVM contracts can call it safely

**Packages:** `core/vm` (`precompiles.go`)

**Notes:** `0x100` native transfer (fixed 3_000 gas) forwards CALLVALUE to a 20-byte recipient; `0x102` staking reserved stub. Flag: `Executor.EnableDewPrecompiles`. Gas table in [Gas and Fees](../execution/gas-and-fees.md).

---

## B4 — Load tests, fee tuning, security audit

**Acceptance:**

- [x] Load / stress tests for PE vs sequential and native path
- [x] Fee policy frozen with documented tuning rationale
- [x] Internal Phase B security checklist + adversarial tests

**Packages:** `tests/load`, `tests/security`, `params`, `docs/security/phase-b-audit.md`

**Notes:** `go test ./tests/load/` and `./tests/security/`; benches under `core/vm` and `core/native`. Fee helpers in `params/fee.go`. Residual items promoted into Phase C or mainnet debt in `agents/debt.md`.

---

## C1 — Mempool admission & fee policy

**Goals:** Bound unconfirmed tx pressure for EVM and DewTx before any public RPC exposure.

**Acceptance:**

- [x] Per-sender and global mempool size limits (configurable)
- [x] Minimum fee / tip checks aligned with [Gas and fees](../execution/gas-and-fees.md) and `params` helpers
- [x] Reject or drop underpriced / oversized payloads without stalling block production
- [x] Unit tests for eviction / replace-by-fee (or documented no-RBF rule)
- [x] DewTx path uses the same admission surface as EVM txs (or explicitly documented dual pools)

**Packages:** `mempool/`, `node/` (`SendRawTransaction` / `SendDewRawTransaction` admit then optional auto-mine)

**Notes:** Unified pool (`mempool.Pool`) for EVM + DewTx. Defaults: global 4096, per-sender 16, max tx 128 KiB, min gas 1 gwei, min tip 1 wei, min Dew fee = `params.MinDewTxFeeWei`, RBF +10% price bump (`PriceBumpPercent=0` disables RBF). Under global pressure, a strictly cheaper pending tx may be evicted for a higher-priced newcomer. **Multi-tx (C1 residual, July 2026):** `DefaultMaxTxsPerBlock=64`; fee auction over continuous per-sender nonce chains; EVM + DewTx nonce-gap queues (`nonce >= account`); auto-mine packs ready pending into one block for both paths. Proposal path skips hard-failing sims and re-selects (`simulateAndFilterLocked`). DewTx remains receipt/index-backed with empty EVM body until a tagged-union body (out of public-testnet-v1 wire freeze).

---

## C2 — Encrypted P2P transport

**Goals:** Replace cleartext dev transport for multi-host and public networks.

**Acceptance:**

- [x] Authenticated encrypted sessions between peers (handshake still binds chain ID + node identity)
- [x] Dev/loopback may keep cleartext behind an explicit flag; default for non-local is encrypted
- [x] Existing gossip, sync, and consensus message types still deliver correctly under encryption
- [x] Integration test: 3+ peers over encrypted transport reach the same committed height

**Packages:** `p2p/` (`secure.go`, `Config.Encrypt`)

**Notes:** Cipher suite: X25519 ECDH + AES-256-GCM; identity handshake after secure hello. Default `Encrypt=true`; cleartext needs `AllowCleartext=true`. Documented in [P2P](../networking/p2p.md).

---

## C3 — SMT state commitment

**Goals:** Replace provisional sorted-leaf state root with Sparse Merkle Tree commitment matching [State](../protocol/state.md).

**Acceptance:**

- [x] `header.StateRoot` is an SMT root over dirty accounts + storage slots after block execution
- [x] Same pre-state + same txs ⇒ identical root on independent nodes
- [x] Migration path or genesis rule documented for chains that used the provisional flat root (dev only OK to wipe)
- [x] Tests cover empty state, single account, storage slots, and delete/empty account cases
- [x] Devnet + PE paths still match sequential roots under the new commitment

**Packages:** `core/state/` (`smt.go`, `IntermediateRoot` → `ComputeSMTRoot`)

**Notes:** Flat KV remains the **hot path**; SMT is commit-time only. Dev nets using provisional roots must re-genesis. Freeze wire meaning of `StateRoot` in C6.

---

## C4 — Staking module (`0x102`)

**Goals:** Real staking surface for validator candidates (and minimal delegation if in scope), replacing the revert stub.

**Acceptance:**

- [x] Precompile `0x102` implements documented ABI (bond / unbond / at least self-stake register)
- [x] State updates respect min self-stake and epoch rules in [Validators](../consensus/validators.md) (public-testnet-v1 freeze in C6)
- [x] Active set / voting power readable for consensus selection (or clear bridge from module state → BFT validator set)
- [x] Fail closed on malformed input; gas schedule documented in [Gas and fees](../execution/gas-and-fees.md)
- [x] Feature flag remains until private testnet operators opt in

**Packages:** `core/native/staking.go`, `core/vm/precompiles.go`, `params/staking.go`, `node` flag

**Notes:** Self-stake only (no liquid staking). `ActiveSet()` ranks candidates; **rotates into BFT** at epoch boundaries when staking on. Jail requires **verified dual-vote** double-sign evidence. **Unbonding period enforced** (queue + `withdraw` / `0x08`). Bond credits immediate CALL payer. Default `EnableStaking=false`.

---

## C5 — Private multi-host testnet & ops

**Goals:** Run a non-loopback private network with restart/chaos discipline and operator docs.

**Acceptance:**

- [x] Documented multi-host topology (3 validators + optional non-validator RPC) beyond in-process `dew devnet`
- [x] Nodes reconnect and sync after process kill / host restart (chaos smoke)
- [x] Runbook stubs: key material locations, enable/disable feature flags, emergency stop
- [x] Encrypted P2P (C2) used on the private net by default
- [x] ERC-20 (or equivalent) deploy + transfer over the multi-host RPC still works

**Packages:** `devnet/` (chaos + encrypt default), docs `ops/private-testnet.md`, `cmd/dew`

**Notes:** Aligns with [Security principles](../security/security-principles.md) “Private testnet” bar. Chaos: `go test ./devnet/ -run Chaos`. No public faucet incentives (C6).

---

## C6 — Public testnet freeze

**Goals:** Freeze parameters and surfaces for the first public testnet; raise the abuse bar.

**Acceptance:**

- [x] Genesis + chain ID + fee floors + precompile addresses documented as freeze candidates (`params/freeze.go`, [public-testnet](../ops/public-testnet.md))
- [x] RPC abuse tests: oversized batches/body, invalid hex, underpriced/oversized spam under C1 limits (`tests/security/c6_rpc_abuse_test.go`)
- [x] Optional fuzz on codec / RPC decode entrypoints (`core/types/dewtx_fuzz_test.go`, `rpc/hexutil_fuzz_test.go`; run with `-fuzz`)
- [x] Docs: freeze table + runbook; residual mainnet-only debt in `agents/debt.md`
- [x] Public testnet runbook: faucet policy, bootnodes process, features on/off

**Packages:** `params/`, `rpc/`, `tests/security/`, docs `ops/public-testnet.md`

**Notes:** Freeze tag **`public-testnet-v1`**. C6 is a **release gate**, not a large feature dump. Path B public surface deployed July 2026 ([live endpoints](../ops/public-testnet.md#live-network-path-b)). Mainnet still requires external audit of consensus + VM bridge + crypto (not C6 acceptance). After C6, prefer config/parameter changes over wire-format churn. RPC limits: 1 MiB body, 100 batch items.

---

## Phase D — product & scale foundation (post freeze)

Post-freeze work is **product / ops / scale foundation**, not a new consensus wire format. **Shipped order** (complete):

```text
D1 Block explorer MVP  →  D2 production faucet  →  D3a multiproc BFT  →  D3b peers  →  durable chaindata  →  product-v1
```

**Remaining D3 workstreams** (D3c staking edges, D3d Path A, D3e audit) are **optional** — not a linear “finish D then mainnet” requirement. Open residuals (PE upgrade, delegation, indexer, etc.) live in [agents/debt.md](../../agents/debt.md) and [upgrades](../product/upgrades.md) until a track owns them.

Open work is listed as **tracks** (one row each) — [Tracks](#tracks-open-work--one-row-each).

---

## D1 — Block explorer MVP

**Goals:** Ship a read-only web UI so public publish can replace `Explorer: (none)` with a live base URL. Spec: [Block explorer (web)](../product/block-explorer.md).

**Acceptance:**

- [x] Package `explorer/` (React + Vite + Tailwind; stack frozen in explorer doc — not under `web/`)
- [x] Env: `PUBLIC_RPC_URL`, `PUBLIC_CHAIN_ID` (`2205`), optional `PUBLIC_EXPLORER_BASE`
- [x] Shell: search, network badge, Dew dark theme, honest RPC / wrong-chain banners
- [x] Home: stats + latest blocks + latest txs (poll ~3–5s while tab visible)
- [x] Routes: `/`, `/block/{n|hash}`, `/tx/{hash}`, `/address/{addr}` with Overview fields from the explorer doc
- [x] Search resolves address / tx hash / block number (and block hash when applicable)
- [x] Failed / pending / not-found states styled; copy + truncate links; mobile stacks without page overflow
- [x] Root scripts: `pnpm explorer:dev` / `explorer:build` / `explorer:preview`
- [x] README + local dev against `dew devnet` / public RPC; no secrets in frontend

**Packages:** `explorer/`, root `package.json` scripts; docs under `docs/product/block-explorer.md`

**Notes:** JSON-RPC only for MVP (no indexer). Do not reimplement state transition in Node. Deploy packaging: `deploy/explorer/` (Dockerfile + compose) and combined `deploy/docker-compose.yml`. Live at `https://explorer-dew.fadosoft.com`. MetaMask base URL and publish template: launch checklist + explorer operator snippet.

---

## D2 — Optional production faucet

**Goals:** Let public users obtain small amounts of test DEW without Anvil keys or unbounded spam.

**Acceptance:**

- [x] Rate limit per IP and/or address (documented operator defaults)
- [x] Captcha or allowlist before open mint
- [x] Small fixed amounts suitable for deploy + a few transfers (not yield)
- [x] Runs **outside** monorepo consensus core (separate service or `scripts`/ops package); disable independently of validators
- [x] Publish template documents faucet URL or `none` / allowlist-only

**Packages:** `faucet/`, `cmd/dewfaucet`, `deploy/faucet.env.example`, `deploy/systemd/dewfaucet.service`; operator guide [Production faucet](../product/faucet.md)

**Notes:** Default public mode **allowlist**; **captcha** (Turnstile/hCaptcha) for open mint; **dev** rate-limit-only for private nets. Live deployment uses **captcha** at `https://faucet-dew.fadosoft.com`. Default drip **1 DEW**; per-address **1/24h**, per-IP **10/h**. Anvil #0 key refused unless `-allow-anvil-key`. C6 residual “Production faucet service” closed by this package.

---

## D3 — Scale when needed (Path A / C4 / audit)

**Goals:** Grow multiproc/private ops foundation first; public multi-host, staking productization, and audit **only when intentionally chosen**. Not a single PR; pick workstreams explicitly. Research or core-feature work may proceed **without** D3d/D3e.

**Design spec (implement from this):** [D3 scale](../scale/d3-scale.md) — multi-process BFT (D3a), peer store (D3b), staking residuals (D3c), Path A (D3d), audit prep (D3e).

**Acceptance (per workstream — do not require all at once):**

### D3a — Multi-process Dew-BFT

- [x] ≥3 `dew run --validator` processes share one canonical chain
- [x] Optional full RPC node syncs commits (no local seal)
- [x] Compose `multi` + ERC-20 smoke path documented
- [x] Encrypted P2P remains default for multiproc
- [x] Min block interval + bulk outbound drop under queue pressure (D3a residual)

### D3b — Peer store / auto-redial

- [x] Durable `<datadir>/peers.json`
- [x] Host auto-redial with backoff after restart
- [x] Documented `--datadir` / compose volumes

### Durable chaindata

- [x] Pebble `<datadir>/chaindata` for headers/bodies/state tip
- [x] Atomic seal + `ImportCommittedBlock` batch with flat state
- [x] `node.Open` restart recovery; genesis mismatch refuses start
- [x] Peers stay in `peers.json` (not merged into chaindata)

### D3c — Staking residuals (C4)

- [x] Unbonding period before withdraw (queue + `0x08` / `0x09`)
- [x] Dual-vote double-sign verify + jail
- [x] Nested CALL bond credits value payer
- [x] ActiveSet → BFT epoch rotation when staking on
- [ ] On-chain slash burn percentages (economics still tentative) — S4 deferred with docs
- [x] Zero-value nested unbond/withdraw call-stack edge — S4 fail-closed (tx.origin) + tests
- [ ] Delegation / commission (deferred unless scoped)

### D3d — Path A multi-host public (optional ops)

- [ ] ≥3 validators on separate hosts with new keys
- [ ] Published public bootnode multiaddrs
- [ ] Full-node public RPC + faucet/explorer pointed at it
- [ ] Launch checklist Path A runbook complete

### D3e — External audit prep (mainnet claims only)

- [ ] Scoped audit pack (consensus + VM bridge + crypto)
- [ ] Threat model + Phase B findings attached
- [ ] Path A or multiproc soak evidence retained
- [ ] Explicit decision recorded before any mainnet claim

**Packages:** `consensus/`, `p2p/`, `node/`, `core/native`, `cmd/dew`, `deploy/`, `devnet/`, operator docs

**Notes:** **D3a done** (July 2026) — `node.Stack`, `--validator` / `--no-auto-mine`, compose `multi` + `node-rpc`; `go test ./devnet/ -run MultiProcessBFT_SharedChain`. ERC-20 compose smoke: `node scripts/devnet-erc20.mjs http://127.0.0.1:8548`. **D3a residual closed** (July 2026) — default `MinBlockInterval` 1s, `--bft.min-block-interval`, bulk P2P drop under queue pressure; heavy soak `DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run MultiProcessBFT_LongEmpty`. **D3b done** (July 2026) — durable `peers.json`, auto-redial with backoff, `--datadir`; tests `./p2p/ -run AutoRedial`. **Durable chaindata done** (July 2026) — Pebble `<datadir>/chaindata`, `node.Open`, restart recovery; [durable-chaindata.md](../ops/durable-chaindata.md); `go test ./node/ -run RestartRecoversTip`. Spec detail: [D3 scale](../scale/d3-scale.md).

**What is next is a choice, not a default:** pick a **track row** — research (R), core PE (4), staking lab (2), product polish (1), or Path A (3). See [Tracks](#tracks-open-work--one-row-each). Prefer config/genesis changes over wire churn under `public-testnet-v1`. Path B auto-mine stays valid as a lab/demo surface. Tokenomics issuance numbers may stay **draft** indefinitely until a mainnet economics freeze is deliberate.

---

## Tracks

Open work after the Phase D foundation. **One track per row** in the [summary table](#tracks-open-work--one-row-each). Ship **one track (or one slice) at a time**. Narrative + status: [Roadmap — Tracks](./roadmap.md#tracks) · full product map: [upgrades](../product/upgrades.md) · residuals: [agents/debt.md](../../agents/debt.md).

**Wire rule (unchanged):** do not change DewTx type, fee floors, precompile addresses, or SMT meaning under `public-testnet-v1` without a documented hardfork.

### Track R — Research lab

| | |
| :--- | :--- |
| **Status** | Partial — H1 PE + H2 multiproc BFT **done** ([research-lab](../ops/research-lab.md)); state optional |
| **Goals** | Measure and explain PE, BFT, state, or staking economics without real mainnet users |
| **Packages** | `tests/load/`, `tests/security/`, `core/vm/`, `devnet/`, `docs/` |

**Acceptance (pick a focus; not all required):**

- [x] At least one written hypothesis (throughput, finality, storage, or staking) — PE wall-clock vs conflict structure ([research-lab.md](../ops/research-lab.md)) (2026-07-13)
- [x] Reproducible harness (load / multiproc / soak command documented) — `tests/load` + `core/vm` Parallel + multiproc BFT ([research-lab.md](../ops/research-lab.md)) (2026-07-13 / 2026-07-14)
- [x] PE: sequential vs parallel matrix (conflict rate, workers, rollback metrics) — `TestLoad_PE_Matrix_ConflictAndWorkers` (2026-07-13)
- [x] BFT: multiproc soak or chaos (commit latency / recovery notes) — H2 + `TestMultiProcessBFT_CommitLatencyLab` / chaos `BFT_CHAOS_ROW` (2026-07-14)
- [ ] State: SMT commit or tip-growth measurement vs flat hot path
- [x] Staking lab (optional): private net with `--staking` scenario notes — [Staking lab S5](../ops/private-testnet.md#staking-lab-s5) (2026-07-14)
- [x] Results recorded under docs or lab notes; code changes keep serial-equivalent tests green — S1–S2 + H2 tables in [research-lab.md](../ops/research-lab.md)

### Track 1 — Product surface

| | |
| :--- | :--- |
| **Status** | Partial — product-v1…v1.2 **done**; deferred polish open |
| **Goals** | Explorer / faucet / Guestbook DX (RPC/UI only) |
| **Packages** | `explorer/`, `faucet-web/`, `examples/guestbook-web/` |

**Acceptance:**

- [x] Product-v1 through v1.2 (explorer P1a–d, faucet P2a–d, Guestbook P3a–b)
- [ ] P1e — indexer / full history / internal txs
- [ ] P1f — verified source / ABI
- [ ] P3c — Guestbook reactions / replies

Detail: [upgrades Track 1](../product/upgrades.md#track-1--product-surface).

### Track 2 — Protocol / D3c staking

| | |
| :--- | :--- |
| **Status** | Partial — MVP **done**; extras open |
| **Goals** | Staking residuals on `0x102` (lab or intentional ops) |
| **Packages** | `core/native/`, `core/vm/precompiles.go`, `params/staking.go` |

**Acceptance:**

- [x] D3c MVP (unbond, double-sign, ActiveSet rotation, nested bond) — see [D3c](#d3c--staking-residuals-c4)
- [ ] Slash burn percentages on-chain — **deferred** (economics draft; S4 note 2026-07-14)
- [ ] Delegation / commission
- [x] Nested unbond/withdraw call-stack edge — **documented fail-closed** (tx.origin; tests 2026-07-14)

### Track 3 — Ops / D3d Path A

| | |
| :--- | :--- |
| **Status** | Optional — open |
| **Goals** | Multi-host **public** BFT when leaving single-host Path B |
| **Packages** | `deploy/`, `cmd/dew`, operator docs |

**Acceptance:**

- [ ] Same open boxes as [D3d](#d3d--path-a-multi-host-public-optional-ops)

### Track 4 — Core node

| | |
| :--- | :--- |
| **Status** | Partial — `dew_getMempoolStats` (S3) + **lazy hydrate** done; PE upgrade open |
| **Goals** | PE, chaindata, mempool/fee telemetry |
| **Packages** | `core/vm/`, `mempool/`, `rpc/`, `node/`, `db/` |

**Acceptance:**

- [x] Mempool / fee telemetry RPC (optional DX) — `dew_getMempoolStats` (2026-07-13)
- [x] Lazy hydrate / log index at large tip — tip-only Open + on-demand block/tx/receipt/logs; optional residual secondary log-index keys (2026-07-14)
- [ ] PE upgrade toward full Block-STM / lower conflict cost (serial-equivalent) — not justified by S2 simple-transfer matrix
- [ ] Other core ergonomics only with docs + tests

### Track 5 — Mainnet gate (D3e)

| | |
| :--- | :--- |
| **Status** | Optional — only before production claims |
| **Goals** | External audit pack + explicit mainnet readiness |
| **Packages** | docs under `security/`, soak evidence, ops logs |

**Acceptance:**

- [ ] Same open boxes as [D3e](#d3e--external-audit-prep-mainnet-claims-only)
- [ ] Explicit mainnet readiness decision recorded
- [ ] Tokenomics issuance numbers frozen if economic claims are made
- [ ] Path A public multi-host if decentralization claims require it (else document Path B limits)
- [ ] No wire-format surprises relative to freeze / hardfork docs

---

## Recommended sequence — research lab → Precompile slots

**Ordered plan (complete through Checkpoint C, 2026-07-14)** — crossed Track R + Track 4 + Track 2, ended at Precompile slots. Track 3 (Path A), Track 5 (audit), and Track 1 indexer (P1e) remain **out of scope** for this sequence.

**End state:** Dew has a documented, test-backed **Precompile slots** registry (`0x01–0x0a` ETH + `0x100+` Dew), active `0x100` / `0x102` correct under lab staking, reserved `0x101` fail-closed, and research notes for PE/BFT/state — still under freeze `public-testnet-v1` (no new live precompile address without hardfork doc).

```mermaid
flowchart LR
  S0[S0 Lab baseline] --> S1[S1 Research harness]
  S1 --> S2[S2 PE matrix]
  S2 --> S3[S3 Telemetry RPC]
  S3 --> S4[S4 0x102 edges]
  S4 --> S5[S5 Staking lab]
  S5 --> S6[S6 Precompile slots]
```

### S0 — Lab baseline

**Goals:** Prove local multiproc + unit surface still green before experiments.

**Acceptance:**

- [x] `go test ./...` green on a clean tree (2026-07-13)
- [x] `go test ./devnet/ -count=1` green (BFT + ERC-20 path) (2026-07-13)
- [ ] Optional: `dew devnet` + `node scripts/smoke-rpc.mjs` chainId **2205**
- [x] Note Path B is optional; do not block on public HTTPS

**Packages:** whole monorepo · **Verify:** commands above · **Scope:** S

### S1 — Research harness + hypothesis

**Goals:** Track R foundation — one written question and a runnable measurement path.

**Acceptance:**

- [x] Hypothesis written (e.g. PE speedup vs conflict rate; or multiproc commit latency) — H1 in [research-lab.md](../ops/research-lab.md) (2026-07-13)
- [x] Document harness commands under `docs/` (ops recipe or short lab note linked from [Track R](#track-r--research-lab))
- [x] Baseline run recorded once (numbers or log path); no code required if existing tests suffice — table in [research-lab.md](../ops/research-lab.md) (2026-07-13)
- [x] Track R “hypothesis” + “reproducible harness” boxes ticked when done

**Packages:** `docs/`, `tests/load/`, `devnet/` · **Verify:** follow the documented commands · **Scope:** S–M  
**Depends on:** S0

### S2 — PE measurement matrix (Track R · Track 4 prep)

**Goals:** Quantify Dew-PE vs sequential; keep serial-equivalent invariant.

**Acceptance:**

- [x] Matrix: sequential vs PE across conflict rates and/or worker counts — `TestLoad_PE_Matrix_ConflictAndWorkers` (2026-07-13)
- [x] Capture rollback / re-exec metrics (`dew_getExecutionStats` or test harness output) — matrix `ROW` logs + [research-lab.md](../ops/research-lab.md)
- [x] Confirm PE roots/receipts match sequential on fixtures (`go test` PE paths green)
- [x] Short results note in docs (table or bullets) — not marketing claims
- [x] Optional follow-up filed only if Block-STM is justified by numbers — **not justified** on simple-transfer matrix; residual left in debt (no Block-STM implementation)

**Packages:** `core/vm/`, `tests/load/`, `docs/execution/parallel-execution.md` · **Verify:** load/PE tests + note · **Scope:** M  
**Depends on:** S1

### Checkpoint A — after S0–S2

- [x] All S0–S2 acceptance boxes checked (2026-07-13)
- [x] No freeze wire changes
- [x] Human review of hypothesis + PE numbers before core feature coding — numbers + H1 recorded in [research-lab.md](../ops/research-lab.md); proceed to S3 telemetry (not Block-STM)

### S3 — Mempool / fee telemetry RPC (Track 4, small)

**Goals:** Observability for lab load without product UI.

**Acceptance:**

- [x] Spec methods under `dew_*` (names, fields, rate/abuse limits aligned with C6) — `dew_getMempoolStats` ([dew-extensions](../api/dew-extensions.md#dew_getmempoolstats)) (2026-07-13)
- [x] Implement read-only telemetry (pool size, per-sender, fee floors, drop/evict counters as available)
- [x] Unit tests + docs in [JSON-RPC](../api/json-rpc.md) / [dew-extensions](../api/dew-extensions.md)
- [x] Does not change admission policy or fee floors

**Packages:** `mempool/`, `rpc/`, `node/`, `docs/api/` · **Verify:** `go test ./rpc/ ./mempool/` · **Scope:** M  
**Depends on:** Checkpoint A

### S4 — Staking precompile edges (`0x102`) (Track 2 / D3c open)

**Goals:** Correctness on live staking methods **in lab only** (`--staking`); no public Path B staking enablement required.

**Acceptance:**

- [x] Nested / zero-value unbond-withdraw actor semantics fixed **or** explicitly documented fail-closed limitation with test coverage of current behavior — **fail-closed tx.origin** + `TestStakingUnbondWithdraw_ActorIsTxOrigin_NestedForwarder` (2026-07-14)
- [x] Tests for bond / unbond / withdraw / jail paths still pass with staking on — includes `TestStakingPrecompile_BondUnbondWithdraw` + existing jail suite
- [x] Slash burn **percentages** either: (a) deferred with economics note, or (b) implemented only after numbers approved in docs — **(a) deferred** ([tokenomics](../economics/tokenomics.md), [slashing](../consensus/slashing.md))
- [x] Delegation / commission **out of scope** for this sequence (leave D3c box open)

**Packages:** `core/vm/precompiles.go`, `core/native/staking.go`, `params/staking.go`, `docs/execution/precompiles.md` · **Verify:** `go test ./core/vm/ ./core/native/ ./consensus/` · **Scope:** M  
**Depends on:** S3 (or S2 if telemetry skipped by choice — prefer S3 first for load lab)

### S5 — Staking lab scenarios (Track R optional hard)

**Goals:** Private-net exercise of ActiveSet + unbond timing + jail, with notes.

**Acceptance:**

- [x] Private / multiproc or in-process path with `SetStakingEnabled(true)` documented — [private-testnet — Staking lab](../ops/private-testnet.md#staking-lab-s5) + `TestStakingLab_Scenario` (2026-07-14)
- [x] Scenario notes: bond → active set rank → unbond wait → withdraw; optional double-sign jail
- [x] Epoch rotation observed or tested when staking on — `TryRotateValidatorSet` at even height in lab genesis
- [x] Public-testnet default remains staking **off** (freeze / launch checklist unchanged) — asserted in test + `params.PublicTestnetStakingOn`

**Packages:** `devnet/`, `docs/ops/private-testnet.md`, `docs/consensus/` · **Verify:** documented commands + tests · **Scope:** M  
**Depends on:** S4

### Checkpoint B — after S3–S5

- [x] Telemetry usable under load (if S3 done) — `dew_getMempoolStats` (S3)
- [x] `0x102` lab behavior documented and tested — S4 edges + S5 lab scenario (2026-07-14)
- [x] No accidental enable of staking on public Path B — default off + freeze checks
- [x] Human OK to proceed to Precompile slots formalization — S0–S5 closed; next **S6**

### S6 — Precompile slots (milestone)

**Goals:** Make [Precompile slots](../protocol/addresses.md#precompile-slots-evm-space) a **first-class registry**: addresses, status, gas, flags, fail-closed reserved slots — code + docs + tests aligned. **Does not** ship a live `0x101` orderbook unless separately approved (default: remain reserved).

**Acceptance:**

#### S6a — Spec / docs

- [x] Expand precompile slot table in [addresses.md](../protocol/addresses.md) (ETH `0x01–0x0a` + Dew range policy) — 2026-07-14
- [x] Align [precompiles.md](../execution/precompiles.md) status table (`0x100` active, `0x101` reserved, `0x102` flagged) — 2026-07-14
- [x] Document **allocation rules**: next free slot, no reuse of retired slots, hardfork required to activate a reserved address — 2026-07-14
- [x] Cross-link freeze table in [public-testnet.md](../ops/public-testnet.md) (addresses frozen; new live module = hardfork) — 2026-07-14
- [x] Note Dew-native space `0xe0…` vs EVM precompile slots (when to use which) — 2026-07-14

#### S6b — Code registry

- [x] Named constants for all Dew slots in use/reserved (`0x100`, `0x101`, `0x102`) in `core/vm` and/or `params` — 2026-07-14
- [x] `0x101` **not** registered in the live precompile map (empty account / fail-closed) — 2026-07-14
- [x] Optional: single `DewPrecompileSlots` registry helper used by executor enablement — 2026-07-14
- [x] Gas constants only for **active** methods; reserved slots have no live gas schedule (or documented TBD only in docs) — 2026-07-14

#### S6c — Tests + DX

- [x] Tests: with Dew precompiles on, `0x100` forwards; `0x101` does not implement swap; `0x102` reverts or no-ops methods when staking off — 2026-07-14
- [x] Tests: with Dew precompiles off, `0x100+` behave as empty accounts (value not forwarded) — 2026-07-14
- [x] Optional lab: explorer or docs list “system contracts” addresses (RPC-only badge OK; no indexer required) — table in [precompiles.md](../execution/precompiles.md) (2026-07-14)
- [x] `agents/debt.md` updated if residuals remain (e.g. implement `0x101` later) — 2026-07-14

**Packages:** `core/vm/precompiles.go`, `params/`, `docs/protocol/addresses.md`, `docs/execution/precompiles.md`, `docs/ops/public-testnet.md`, tests under `core/vm/`, `tests/security/` · **Verify:** `go test ./core/vm/ ./params/ ./tests/security/` + docs review · **Scope:** M  
**Depends on:** Checkpoint B

### Checkpoint C — Precompile slots done

- [x] All S6a–S6c boxes checked — 2026-07-14
- [x] Freeze still holds: no new **active** precompile address on public-testnet-v1 without hardfork doc — `0x101` reserved only
- [x] Track R + Track 4 telemetry + `0x102` edges + Precompile slots registry complete for this sequence — 2026-07-14
- [ ] Next work (optional, **new** plan): full Block-STM, delegation, `0x101` orderbook design, or Path A — each needs its own approval

### Out of scope (this sequence)

| Item | Why deferred |
| :--- | :--- |
| D3d Path A public multi-host | Ops/public users; not research path |
| D3e external audit | Mainnet claims only |
| P1e indexer / P1f verified source | Product polish |
| Live `0x101` swap/orderbook | Needs separate design + hardfork or lab-only flag approval |
| Delegation / commission | Large staking product; after Precompile slots |
| Tokenomics issuance freeze | Mainnet economics |

### Risks

| Risk | Mitigation |
| :--- | :--- |
| Scope creep into Block-STM during S2 | Measure first; implement only if numbers demand |
| Inventing slash % / fees | Keep draft; require doc approval before code |
| Enabling staking on Path B by accident | Default off; checklist + tests |
| Activating `0x101` “while we’re here” | S6 explicitly reserved-only unless new plan |
