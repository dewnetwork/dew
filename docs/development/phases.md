---
title: Implementation Phases
description: Acceptance criteria for each build phase.
category: development
order: 30
status: draft
---

# Implementation Phases

Each phase should leave the **monorepo buildable and testable** (Go packages and any Node scripts that phase introduces). Prefer small PRs per phase.

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

- [ ] Same fixtures as sequential (roots + receipts)
- [ ] Speedup on non-conflicting workloads
- [ ] Metrics for rollback rate

---

## B2 — DewTx

**Acceptance:**

- [ ] Codec + signature domain frozen
- [ ] `dew_sendRawTransaction` works
- [ ] Fail closed on incomplete access lists

---

## B3 — Native modules / precompiles

**Acceptance:**

- [ ] At least one useful precompile behind feature flag
- [ ] Gas/fee schedule documented
- [ ] EVM contracts can call it safely
