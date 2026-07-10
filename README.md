# Dewchain

High-performance, EVM-compatible Layer 1 blockchain — built **from scratch** in a **monorepo** using **Go + Node.js**.

| Goal | Approach |
| :--- | :--- |
| **Faster** than Ethereum | ~1s blocks, flat state, later parallel execution |
| **More secure** | Dew-BFT instant finality + slashing |
| **Cheaper** | Higher capacity, discounted storage gas, later native micro-fees |
| **ETH-first** | Solidity / MetaMask / Foundry path before Dew-native features |

## Monorepo

This repository is a **single monorepo** for the whole Dewchain stack:

| Stack | Role |
| :--- | :--- |
| **Go** | Core L1: node (`dewchain`), CLI (`dewcli`), consensus, P2P, state, EVM bridge, JSON-RPC server |
| **Node.js** | **Build and serve the documentation website** from `docs/`; plus scripts, localnet helpers, SDK/RPC tests, deploy utilities |

Protocol logic lives in Go. Node is for docs web + developer tooling — not a second consensus client.

Layout details: [docs/development/go-project-layout.md](./docs/development/go-project-layout.md).

## Documentation

Full protocol and build docs: [`docs/`](./docs/README.md).

**Docs website (VitePress):**

```bash
pnpm install
pnpm docs:dev      # local preview (default http://localhost:5173)
pnpm docs:build    # static site → docs/.vitepress/dist
pnpm docs:preview  # serve production build
```

Markdown under `docs/` is the source of truth; Node only builds the site.

**Start here:**

1. [Vision](./docs/overview/vision.md)
2. [Design principles](./docs/overview/design-principles.md)
3. [Roadmap](./docs/development/roadmap.md)
4. [Implementation phases](./docs/development/phases.md)

## Status

Monorepo scaffold + documentation stage. **Docs website** (`docs:dev` / `docs:build`) is wired. Go packages and remaining Node tooling land phase-by-phase per the roadmap.

## License

TBD
