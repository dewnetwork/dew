---
title: Dew Documentation
description: Index of Dew protocol, architecture, and development docs.
category: root
order: 0
status: stable
---

# Dew Documentation

**Dew** is a high-performance, EVM-compatible Layer 1 blockchain built **from scratch** in a **monorepo** with **Go + Node.js**.

| Goal | Meaning |
| :--- | :--- |
| **Faster than Ethereum** | Sub-second blocks, parallel execution, flat state + SMT commit |
| **More secure** | BFT instant finality, clear slashing, minimal trusted surface |
| **Cheaper than Ethereum** | Higher capacity, discounted storage gas, EIP-1559 burn, Dew-native micro-fees |
| **ETH-compatible first** | MetaMask, Solidity, Hardhat/Foundry work out of the box |

## Monorepo (Go + Node)

| Stack | Owns |
| :--- | :--- |
| **Go** | Chain node, CLI, consensus, P2P, state, EVM, JSON-RPC, ops faucet |
| **Node.js** | Docs website (VitePress from this tree); landing; explorer + faucet-web; scripts |

Markdown under `docs/` is the source of truth. **Node builds the web docs with VitePress** (`pnpm docs:dev` / `pnpm docs:build`). See [Monorepo layout](./build/go-project-layout.md).

## Product status

```
A — Ethereum parity          done
B — Dew-PE / DewTx / native  done
C — Testnet readiness        done (public-testnet-v1 freeze)
D — Product surface + scale  D1–D2 + D3a/D3b + durable chaindata done;
                             D3c–D3e on demand
```

**public-testnet-v1 is live** on path B (July 2026): RPC `https://rpc-dew.fadosoft.com`, explorer `https://explorer-dew.fadosoft.com`, faucet `https://faucet-dew.fadosoft.com`. Details: [Public testnet freeze](./ops/public-testnet.md).

## Categories

| Category | Path | What you will find |
| :--- | :--- | :--- |
| [Overview](./overview/) | `docs/overview/` | Vision, principles, ETH compatibility |
| [Architecture](./architecture/) | `docs/architecture/` | Node components, lifecycle |
| [Protocol](./protocol/) | `docs/protocol/` | Crypto, addresses, txs, blocks, state |
| [Execution](./execution/) | `docs/execution/` | EVM, gas, precompiles, PE, Dew-native |
| [Consensus](./consensus/) | `docs/consensus/` | Dew-BFT, validators, slashing |
| [Networking](./networking/) | `docs/networking/` | P2P, gossip, sync |
| [API](./api/) | `docs/api/` | JSON-RPC (`eth_*`, `dew_*`) |
| [Economics](./economics/) | `docs/economics/` | Tokenomics, genesis |
| [Build](./build/) | `docs/build/` | Monorepo layout, roadmap, phases |
| [Networks & ops](./ops/) | `docs/ops/` | Devnet, private/public testnet, launch, chaindata |
| [Product surface](./product/) | `docs/product/` | Explorer, faucet |
| [Scale](./scale/) | `docs/scale/` | D3 workstreams (done vs pending) |
| [Security](./security/) | `docs/security/` | Threat model, principles, Phase B audit |

## Builder quick start

1. [Quick start (5 minutes)](./ops/quickstart.md) — `dew devnet` + MetaMask + Foundry ERC-20  
2. Sample project: [examples/foundry](../examples/foundry/)  
3. Live public RPC: [Public testnet freeze](./ops/public-testnet.md)

## Suggested reading order

1. [Vision](./overview/vision.md)
2. [Design principles](./overview/design-principles.md)
3. [Architecture overview](./architecture/overview.md)
4. [Public testnet freeze](./ops/public-testnet.md) (if operating or integrating)
5. [Roadmap](./build/roadmap.md) → [Phases](./build/phases.md)
6. Protocol → Execution → Consensus → Networking → API as needed

## Diagrams & charts (Mermaid)

The docs site renders Mermaid fenced blocks. Prefer a diagram when prose alone is hard to scan.

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
| [Gossip and sync](./networking/gossip-and-sync.md) | Inventory sequence; catch-up |
| [Roadmap](./build/roadmap.md) | Phase A–D dependency graph |
| [D3 scale](./scale/d3-scale.md) | Workstream map |
| [Monorepo layout](./build/go-project-layout.md) | Docs web build flow |

## Frontmatter convention (docs web)

```yaml
---
title: Page title
description: One-line summary for SEO and sidebars
category: overview | architecture | protocol | execution | consensus | networking | api | economics | build | ops | product | scale | security
order: 10
status: draft | stable
---
```

| Concern | Where |
| :--- | :--- |
| Source markdown | `docs/**/*.md` |
| Sidebar order | frontmatter `order` + [`sidebar.yaml`](./sidebar.yaml) |
| Site config / theme | `docs/.vitepress/` |
| Build / preview | `pnpm docs:dev` · `docs:build` · `docs:preview` |
| Hosting output | `docs/.vitepress/dist` (merged into root `dist/` via `site:build`) |

## Spec status

Phase **C6** freezes **public-testnet-v1** wire and fee surfaces (`params/freeze.go`). Numbers in the freeze table should not churn without a documented re-genesis / hardfork note. Residual `_tentative_` (e.g. tokenomics issuance) is mainnet- or economics-review scope.

## Repository layout

Full tree and ownership: [Monorepo layout](./build/go-project-layout.md).
