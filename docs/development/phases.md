---
title: Implementation Phases
description: Acceptance criteria for Phases A–C (compat, native/PE, testnet readiness).
category: development
order: 30
status: draft
---

# Implementation Phases

Each phase should leave the **monorepo buildable and testable** (Go packages and any Node scripts that phase introduces). Prefer small PRs per phase.

| Band | Status | Theme |
| :--- | :----- | :---- |
| **A1–A7** | Done | ETH-compatible L1 + local multi-validator devnet |
| **B1–B4** | Done | Dew-PE, DewTx, precompiles, load/security baselining |
| **C1–C6** | C1–C5 done; C6 planned | Mempool, encrypted P2P, SMT, staking, private → public testnet |

High-level order: [Roadmap](./roadmap.md).

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

**Notes:** Header hash = `Keccak-256(RLP(header fields))`. Flat state root is a provisional sorted-leaf commitment (SMT lands later). Sample `genesis.json` at repo root; load via `config.LoadGenesisFile`.

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
- [x] MetaMask shows balance _(dev: connect http://127.0.0.1:8545, chainId 2026)_
- [x] Foundry/Hardhat deploy succeeds against local node _(dev auto-mine per tx)_
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

**Notes:** TCP framing `uint32be length || uint8 type || payload` with RLP payloads. Signed handshake (chain ID, height, node key). Inventory/GetData gossip for txs and blocks; `GetBlocks` range sync. Consensus channel `0x10–0x12` floods proposals/votes. Cleartext suitable for private devnets; encrypted transport later.

---

## A7 — Devnet

**Goals:** End-to-end demo.

**Acceptance:**

- [x] `genesis.json` + init instructions
- [x] 3 validators, 1 RPC node
- [x] Deploy and use ERC-20 over RPC
- [x] Document chain ID, ports, faucet

**Packages:** `devnet/`, `cmd/dew` (`init`, `devnet`), `scripts/devnet-erc20.mjs`

**Notes:** `dew init` writes genesis with 3 Anvil-compatible validators + faucet alloc. `dew devnet` starts in-process LocalCluster BFT, loopback P2P mesh, and JSON-RPC (default `:8545`). ERC-20 fixture over RPC covered by `go test ./devnet/`. Operator guide: [Local Devnet](./devnet.md).

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

**Packages:** `mempool/`, `node/` (`SendRawTransaction` / `SendDewRawTransaction` admit then auto-mine)

**Notes:** Unified pool (`mempool.Pool`) for EVM + DewTx. Defaults: global 4096, per-sender 16, max tx 128 KiB, min gas 1 gwei, min tip 1 wei, min Dew fee = `params.MinDewTxFeeWei`, RBF +10% price bump (`PriceBumpPercent=0` disables RBF). Under global pressure, a strictly cheaper pending tx may be evicted for a higher-priced newcomer. Full fee auction still deferred.

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
- [x] State updates respect min self-stake and epoch rules in [Validators](../consensus/validators.md) (_tentative_ numbers OK until C6 freeze)
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

**Packages:** `devnet/` (chaos + encrypt default), `docs/development/private-testnet.md`, `cmd/dew`

**Notes:** Aligns with [Security principles](../security/security-principles.md) “Private testnet” bar. Chaos: `go test ./devnet/ -run Chaos`. No public faucet incentives (C6).

---

## C6 — Public testnet freeze

**Goals:** Freeze parameters and surfaces for the first public testnet; raise the abuse bar.

**Acceptance:**

- [ ] Genesis + chain ID + fee floors + precompile addresses documented as freeze candidates (update `_tentative_` where ready)
- [ ] RPC abuse tests: oversized batches, invalid hex, spam sendRawTransaction under C1 limits
- [ ] Optional external fuzzing pass on codec / RPC entrypoints (or scheduled with owners)
- [ ] Docs: relevant pages move from pure draft toward “testnet freeze” notes; residual mainnet-only debt listed in `agents/debt.md`
- [ ] Public testnet runbook: faucet policy, bootnodes, expected features on/off

**Packages:** `config/`, `params/`, `tests/security/`, `docs/` (genesis, economics, API)

**Notes:** C6 is a **release gate**, not a large feature dump. Mainnet still requires external audit of consensus + VM bridge + crypto (not C6 acceptance). After C6, prefer config/parameter changes over wire-format churn.
