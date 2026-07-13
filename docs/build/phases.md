---
title: Implementation Phases
description: Acceptance criteria for Phases A–D (compat, native/PE, testnet, product surface).
category: build
order: 30
status: stable
---

# Implementation Phases

Each phase should leave the **monorepo buildable and testable** (Go packages and any Node scripts that phase introduces). Prefer small PRs per phase.

| Band | Status | Theme |
| :--- | :----- | :---- |
| **A1–A7** | Done | ETH-compatible L1 + local multi-validator devnet |
| **B1–B4** | Done | Dew-PE, DewTx, precompiles, load/security baselining |
| **C1–C6** | **Done** (C6 = public-testnet-v1 freeze) | Mempool, encrypted P2P, SMT, staking, private → public freeze |
| **D1–D3** | **D1–D2 done** · D3a/D3b + durable chaindata done · D3c–D3e pending | Product surface + optional ops scale after public-testnet-v1 |

High-level order: [Roadmap](./roadmap.md).

**Ops note:** **public-testnet-v1 is live** on path B (July 2026) — see [Public testnet freeze](../ops/public-testnet.md#live-network-path-b). Private soak and launch checklist A–B remain the operator runbook for new hosts. Phase D assumes a live or local RPC (`chainId` **2205`) and does **not** re-open the C6 wire freeze.

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

**Notes:** Unified pool (`mempool.Pool`) for EVM + DewTx. Defaults: global 4096, per-sender 16, max tx 128 KiB, min gas 1 gwei, min tip 1 wei, min Dew fee = `params.MinDewTxFeeWei`, RBF +10% price bump (`PriceBumpPercent=0` disables RBF). Under global pressure, a strictly cheaper pending tx may be evicted for a higher-priced newcomer. **Multi-tx (C1 residual, July 2026):** `DefaultMaxTxsPerBlock=64`; fee auction over continuous per-sender nonce chains; EVM nonce-gap queue (`nonce >= account`); auto-mine packs ready pending into one block. DewTx auto-mine still one-tx-per-seal.

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

**Notes:** Self-stake only (no liquid staking). `ActiveSet()` ranks candidates for BFT selection. Jail requires non-zero evidence hash (placeholder until full double-sign verify). Unbonding period not fully enforced — residual. Default `EnableStaking=false`.

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

## Phase D — After public-testnet-v1

Post-freeze work is **product / ops / scale**, not a new consensus wire format. Order:

```text
D1 Block explorer MVP  →  D2 optional faucet  →  D3 Path A / C4 residuals / audit when scaling
```

Residuals that stay open across D (staking, multi-process BFT, PE upgrade) remain in [agents/debt.md](../../agents/debt.md) until a D-step owns them.

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

**Goals:** Grow beyond single-host controlled RPC and self-stake stubs only when product demand requires it. Not a single PR; pick workstreams explicitly.

**Design spec (implement from this):** [D3 scale](../scale/d3-scale.md) — multi-process BFT (D3a), peer store (D3b), staking residuals (D3c), Path A (D3d), audit prep (D3e).

**Acceptance (per workstream — do not require all at once):**

| Workstream | Done when |
| :--- | :--- |
| **D3a — C5 multi-process BFT** | ≥3 `dew run --validator` processes share one canonical chain; optional full RPC syncs commits; compose `multi` + ERC-20 smoke ([d3-scale](../scale/d3-scale.md#d3a--multi-process-dew-bft)) |
| **D3b — peer store / redial** | Restart recovery without manual redial; documented data dir ([d3-scale](../scale/d3-scale.md#d3b--peer-store-and-auto-redial)) — **done** July 2026 (`peers.json`, Host redial loop, `--datadir`) |
| **D3c — C4 staking residuals** | Unbonding enforced; double-sign evidence; ActiveSet → BFT epoch rotation ([d3-scale](../scale/d3-scale.md#d3c--staking-residuals-c4), [debt](../../agents/debt.md) C4) |
| **D3d — Path A multi-host public** | ≥3 validators + optional RPC; new keys; bootnodes published ([launch-checklist](../ops/launch-checklist.md) path A, [d3-scale](../scale/d3-scale.md#d3d--path-a-multi-host-public)) |
| **D3e — external audit** | Scoped audit pack before mainnet ([phase-b-audit](../security/phase-b-audit.md), [d3-scale](../scale/d3-scale.md#d3e--external-audit-prep)) |

**Packages:** `consensus/`, `p2p/`, `node/`, `core/native`, `cmd/dew`, `deploy/`, `devnet/`, operator docs

**Notes:** **D3a done** (July 2026) — `node.Stack`, `--validator` / `--no-auto-mine`, compose `multi` + `node-rpc`; `go test ./devnet/ -run MultiProcessBFT_SharedChain`. ERC-20 compose smoke: `node scripts/devnet-erc20.mjs http://127.0.0.1:8548`. **D3a residual closed** (July 2026) — default `MinBlockInterval` 1s, `--bft.min-block-interval`, bulk P2P drop under queue pressure; heavy soak `DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run MultiProcessBFT_LongEmpty`. **D3b done** (July 2026) — durable `peers.json`, auto-redial with backoff, `--datadir`; tests `./p2p/ -run AutoRedial`. **Durable chaindata done** (July 2026) — Pebble `<datadir>/chaindata`, `node.Open`, restart recovery; [durable-chaindata.md](../ops/durable-chaindata.md); `go test ./node/ -run RestartRecoversTip`. Next: [D3d Path A](../scale/d3-scale.md#d3d--path-a-multi-host-public) or D3c when needed. Prefer config/genesis changes over wire churn under `public-testnet-v1`. Path B auto-mine deployment stays valid until operators migrate. Tokenomics issuance numbers may stay draft until mainnet.
