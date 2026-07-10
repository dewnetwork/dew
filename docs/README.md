---
title: Dewchain Documentation
description: Index of Dewchain protocol, architecture, and development docs.
category: root
order: 0
status: draft
---

# Dewchain Documentation

**Dewchain** is a high-performance, EVM-compatible Layer 1 blockchain built **from scratch** in a **monorepo** with **Go + Node.js**.

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

Markdown under `docs/` is the source of truth. **Node is required to build the web docs** (e.g. VitePress / Nextra / Docusaurus — tool TBD). Frontmatter on every page is ready for that site. See [Monorepo layout](./development/go-project-layout.md).

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
| Sidebar order | frontmatter `order` + `sidebar.yaml` |
| Build / preview site | **Node.js** (`package.json` scripts such as `docs:dev`, `docs:build` — when wired) |
| Hosting output | static `dist` / `.vitepress/dist` / equivalent (tool-specific) |

## Spec status

All documents are **`status: draft`** until the first public testnet freezes wire formats and genesis parameters. Numbers marked _tentative_ may change before freeze.

## Repository layout

This project is a **monorepo**. Docs live under `docs/` (source of truth); **Node builds the web docs** from that tree; Go owns the chain. Full tree and ownership: [Monorepo layout](./development/go-project-layout.md).
