---
title: Dew Documentation
description: Index of Dew protocol, architecture, and development docs.
category: root
order: 0
status: draft
---

# Dew Documentation

**Dew** is a high-performance, EVM-compatible Layer 1 blockchain built **from scratch** in a **monorepo** with **Go + Node.js**.

| Goal                      | Meaning                                                               |
| :------------------------ | :-------------------------------------------------------------------- |
| **Faster than Ethereum**  | Sub-second blocks, parallel execution, flat state DB                  |
| **More secure**           | BFT instant finality, clear slashing, minimal trusted surface         |
| **Cheaper than Ethereum** | Lower gas for storage/ops, EIP-1559 burn, Dew-native micro-fees later |
| **ETH-compatible first**  | MetaMask, Solidity, Hardhat/Foundry work out of the box               |

## Monorepo (Go + Node)

| Stack | Owns |
| :--- | :--- |
| **Go** | Chain node, CLI, consensus, P2P, state, EVM, JSON-RPC server |
| **Node.js** | **Docs website** (build/preview static site from this `docs/` tree); scripts, localnet, RPC smoke tests, later SDK |

Markdown under `docs/` is the source of truth. **Node builds the web docs with VitePress** (`pnpm docs:dev` / `pnpm docs:build` from the monorepo root). Frontmatter on every page drives titles, SEO, and navigation. See [Monorepo layout](./development/go-project-layout.md).

## Product strategy

```
Phase A — Ethereum parity (ship first)
  Crypto · Types · Flat state · EVM · JSON-RPC · BFT · P2P · Devnet

Phase B — Dew advantages (after parity)
  Parallel execution (Dew-PE) · Dew-native txs · Native modules · Precompiles
```

Do **not** block Phase A on parallel execution or Dew-native formats. Compatibility and a correct single-node → multi-validator path come first.

## Categories

| Category                        | Path                 | What you will find                          |
| :------------------------------ | :------------------- | :------------------------------------------ |
| [Overview](./overview/)         | `docs/overview/`     | Vision, principles, ETH compatibility scope |
| [Architecture](./architecture/) | `docs/architecture/` | Node components, system diagram             |
| [Protocol](./protocol/)         | `docs/protocol/`     | Crypto, addresses, txs, blocks, state       |
| [Execution](./execution/)       | `docs/execution/`    | EVM, gas, precompiles, parallel, Dew-native |
| [Consensus](./consensus/)       | `docs/consensus/`    | Dew-BFT, validators, slashing               |
| [Networking](./networking/)     | `docs/networking/`   | P2P, gossip, sync                           |
| [API](./api/)                   | `docs/api/`          | JSON-RPC (`eth_*`, later `dew_*`)           |
| [Economics](./economics/)       | `docs/economics/`    | Tokenomics, genesis                         |
| [Development](./development/)   | `docs/development/`  | Monorepo layout, roadmap, phased build      |
| [Security](./security/)         | `docs/security/`     | Threat model, security principles           |

## Suggested reading order

1. [Vision](./overview/vision.md)
2. [Design principles](./overview/design-principles.md)
3. [Architecture overview](./architecture/overview.md)
4. [Ethereum compatibility](./overview/ethereum-compatibility.md)
5. [Development roadmap](./development/roadmap.md)
6. Protocol → Execution → Consensus → Networking → API as needed

## Diagrams & charts (Mermaid)

The docs site renders [Mermaid](https://mermaid.js.org/) fenced blocks (` ```mermaid `). Prefer a diagram when prose alone is hard to scan: flows, state machines, sequences, splits, and comparisons.

| Page | Visualization |
| :--- | :--- |
| [Vision](./overview/vision.md) | Build phases; product goal quadrant |
| [Architecture overview](./architecture/overview.md) | Node components; tx sequence; roles |
| [Node internals](./architecture/node-layout.md) | Process lifecycle |
| [State](./protocol/state.md) | Flat KV hot path vs SMT commit |
| [Execution overview](./execution/overview.md) | Pipeline; Phase A vs B |
| [Parallel execution](./execution/parallel-execution.md) | Dew-PE validate / re-exec loop |
| [Gas and fees](./execution/gas-and-fees.md) | EIP-1559 fee split |
| [Dew-BFT](./consensus/dew-bft.md) | Round state machine |
| [Validators](./consensus/validators.md) | Epoch active-set rotation |
| [Gossip and sync](./networking/gossip-and-sync.md) | Inventory sequence; catch-up |
| [Tokenomics](./economics/tokenomics.md) | Reward pie; supply flow |
| [Threat model](./security/threat-model.md) | Assets mindmap; adversary map |
| [Roadmap](./development/roadmap.md) | Phase A / B dependency graphs |
| [Monorepo layout](./development/go-project-layout.md) | Docs web build flow |

**Author tip:** keep ASCII only when a one-liner is clearer; otherwise use Mermaid so the web site can render interactively. Theme colors follow Dew brand via VitePress config.

## Frontmatter convention (docs web)

Every page uses YAML frontmatter so a **Node-based docs site** can render sidebars, titles, and SEO metadata:

```yaml
---
title: Page title
description: One-line summary for SEO and sidebars
category: overview | architecture | protocol | execution | consensus | networking | api | economics | development | security
order: 10 # sort within category (lower first)
status: draft | stable
---
```

Category folders also include `_category.md` for sidebar labels and ordering. Navigation map: [`sidebar.yaml`](./sidebar.yaml).

| Concern | Where |
| :--- | :--- |
| Source markdown | `docs/**/*.md` |
| Sidebar order | frontmatter `order` + `sidebar.yaml` (VitePress loads `sidebar.yaml`) |
| Site config / theme | `docs/.vitepress/` |
| Build / preview site | **Node.js (pnpm)** — `pnpm docs:dev` · `docs:build` · `docs:preview` |
| Hosting output | `docs/.vitepress/dist` |

## Spec status

All documents are **`status: draft`** until the first public testnet freezes wire formats and genesis parameters. Numbers marked _tentative_ may change before freeze.

## Repository layout

This project is a **monorepo**. Docs live under `docs/` (source of truth); **Node builds the web docs** from that tree; Go owns the chain. Full tree and ownership: [Monorepo layout](./development/go-project-layout.md).
