---
title: Design Principles
description: Engineering principles that guide protocol and implementation choices.
category: overview
order: 20
status: stable
---

# Design Principles

## 1. Ethereum compatibility first

Phase A must work with existing wallets and tooling. Prefer standard:

- secp256k1 + Keccak-256 addresses
- EIP-1559-style transactions and fees
- `go-ethereum` EVM embedding where practical
- JSON-RPC `eth_*` / `net_*` / `web3_*`

Dew-specific features must not break the default Ethereum path.

## 2. One monorepo, two runtimes (Go + Node)

Dew is a **monorepo**:

- **Go** — canonical L1 (node, consensus, P2P, state, EVM, RPC server, CLI)
- **Node.js** — docs site (`docs/`), explorer, faucet UI, Guestbook SPA, smoke scripts

Do not split node / docs / product surfaces into separate product repos without a strong reason. Node must not reimplement consensus or state transition. For the chain it only talks to the Go node (RPC / process orchestration).

## 3. Build from scratch, reuse battle-tested pieces

The **node, consensus, p2p, and state pipeline** are Dew-owned Go code.  
Libraries that are hard to get right (EVM interpreter, secp256k1, RLP) may be reused from mature implementations (e.g. go-ethereum packages) behind clean interfaces.

## 4. Separate execution from commitment

- **Execution path**: flat DB + in-memory caches (and later MVCC for parallel exec)
- **Commitment path**: Sparse Merkle Tree (or equivalent) producing `StateRoot` for the block header

Execution must not pay MPT traversal cost on every `SLOAD`/`SSTORE`. The state root is still computed **before** consensus commit so validators can verify deterministically.

## 5. Instant finality over longest-chain

Dew-BFT targets **one-block finality**. No probabilistic reorg game for dapps under normal conditions. Safety relies on voting power and slashing, not uncle/orphan economics.

## 6. Phase complexity

| Layer | Base (shipped) | Optional / later |
| :--- | :--- | :--- |
| Transactions | EVM EIP-1559 + legacy | + `DewTx` (`0xdf`) |
| Execution | Sequential EVM | Dew-PE (fork+overlay; serial-equivalent) |
| RPC | `eth_*` / `net_*` / `web3_*` | + `dew_*` native submit + metrics |
| Precompiles | Cancun set | + `0x100` / `0x102` (staking flag off by default) |
| Consensus | Dew-BFT multiproc or Path B auto-mine | Path A multi-host public; stake-weighted set rotation (D3c) |

Do not couple consensus correctness to parallel scheduler sophistication.

## 7. Explicit economics

Fees, inflation, staking minimums, and slash percentages must live in **normative docs and genesis config**, not only in marketing copy. Prefer boring, auditable formulas.

## 8. Security by reduction of surprises

- Deterministic state transition for a given block
- Clear wire formats and message types
- Fail closed on invalid signatures, wrong chain ID, or consensus timeout
- Prefer simple protocols (inventory gossip, BFT rounds) over clever unpublished schemes

## 9. Operational model (Go node + Node docs/tooling)

- Go binaries: `dew`, `dewcli`, `dewfaucet`, `dewindex`
- Node.js: VitePress docs, explorer, faucet-web, guestbook-web, smoke scripts
- Config via files + flags; durable chaindata under `--datadir`
- Tests as acceptance (Go unit/integration/devnet + optional Node RPC checks)
