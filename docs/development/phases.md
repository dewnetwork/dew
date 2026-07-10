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

- [ ] HTTP RPC on 8545
- [ ] Methods in [JSON-RPC required list](../api/json-rpc.md)
- [ ] MetaMask shows balance
- [ ] Foundry/Hardhat deploy succeeds against local node
- [ ] Optional: Node smoke script under monorepo `scripts/` hits `eth_chainId`

**Packages:** `rpc/`, `cmd/dew` (+ Node test script if added)

---

## A5 — Dew-BFT (local)

**Goals:** Propose and commit blocks with votes.

**Acceptance:**

- [ ] Round state machine advances height
- [ ] Single validator auto-commits (dev mode)
- [ ] Three local validators reach commit with \(>2/3\) votes
- [ ] Invalid root → prevote nil

**Packages:** `consensus/`

---

## A6 — P2P

**Goals:** Multi-machine capable networking.

**Acceptance:**

- [ ] Handshake + peer store
- [ ] Tx and block gossip
- [ ] Sync from height 0 behind peer
- [ ] Consensus messages delivered among validators

**Packages:** `p2p/`

---

## A7 — Devnet

**Goals:** End-to-end demo.

**Acceptance:**

- [ ] `genesis.json` + init instructions
- [ ] 3 validators, 1 RPC node
- [ ] Deploy and use ERC-20 over RPC
- [ ] Document chain ID, ports, faucet

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
