---
title: Monorepo Layout
description: Dew monorepo — Go L1 core plus Node.js for docs website and tooling.
category: build
order: 10
status: stable
---

# Monorepo Layout

Dew is a **single monorepo**: one git repository, one product surface, shared docs and CI.

## Stack split

| Language | Responsibility | Package manager |
| :--- | :--- | :--- |
| **Go** | L1 node, consensus, P2P, state, EVM, RPC, `dewcli`, `dewfaucet` | `go.mod` / `go.sum` at repo root |
| **Node.js** | Docs website (VitePress from `docs/`); landing (`web/`); explorer + faucet-web SPAs; scripts | `package.json` + `pnpm-lock.yaml` at repo root |

### Rules of ownership

- **Canonical chain logic is Go.** Validators and full nodes run Go binaries only.
- **Node does not reimplement consensus or state transition.** It consumes the Go node via JSON-RPC.
- **Docs source is `docs/`.** Node only *builds and serves* the website from that tree.
- Prefer **one root** for each stack (`go.mod`, `package.json`) unless a future workspace split is justified.

## Repository tree (current)

```
dew/                            # monorepo root
├── cmd/
│   ├── dew/                    # full node (run, init, devnet)
│   ├── dewcli/                 # wallet CLI
│   └── dewfaucet/              # production faucet process
├── core/
│   ├── types/                  # Header, block, tx, receipt, DewTx
│   ├── state/                  # Flat state + SMT commit
│   ├── vm/                     # EVM bridge, PE, precompiles
│   └── native/                 # DewTx executor, staking
├── crypto/                     # secp256k1, keccak, keystore
├── db/                         # Database interface + Pebble only
├── mempool/                    # Unified EVM + DewTx admission pool
├── consensus/                  # Dew-BFT engine, builder, runner
├── p2p/                        # Host, gossip, sync, redial, peers.json
├── node/                       # Backend, Stack, Open/chaindata, import
├── rpc/                        # JSON-RPC HTTP server
├── config/                     # Genesis load
├── params/                     # Chain constants, fees, freeze, staking
├── faucet/                     # Faucet library (ops HTTP)
├── devnet/                     # Multi-validator + multiproc BFT tests
├── tests/                      # load + security suites
├── deploy/                     # Docker Compose, systemd, nginx samples
├── examples/                   # Foundry + Hardhat samples + Guestbook SPA
├── scripts/                    # Node/shell helpers (smoke-rpc, build-site)
├── web/                        # Astro marketing landing
├── explorer/                   # Block explorer SPA (React/Vite)
├── faucet-web/                 # Faucet UI SPA
├── docs/                       # Protocol + engineering docs (this tree)
│   ├── build/ ops/ product/ scale/ …
│   ├── sidebar.yaml
│   └── .vitepress/             # VitePress config + theme
├── go.mod
├── package.json                # pnpm root scripts
└── README.md
```

Optional later (not present today): `packages/sdk`, `packages/localnet`.

## Go packages

| Rule | Why |
| :--- | :--- |
| `cmd/*` is thin | Business logic in libraries |
| No import cycles | Use interfaces at boundaries |
| `params` has no heavy deps | Constants only |
| Codecs versioned | Avoid silent fork of encodings |
| Disk via Pebble | Single `db.PebbleDB` backend; tests use `db.OpenTest` |

- Go **1.25+** (see root `go.mod`; `toolchain go1.25.12` for patched stdlib; CI uses `go-version: 1.25.x`)
- EVM via **go-ethereum v1.17.x** (`core/vm` Bridge implements geth `vm.StateDB`)
- Unit tests next to packages; multi-node / soak under `devnet/` and `tests/`

## Node packages

| Use | Role |
| :--- | :--- |
| **Docs website** | VitePress from `docs/**/*.md` |
| Landing | `web/` Astro site |
| Explorer / faucet UI | `explorer/`, `faucet-web/` |
| Scripts | `scripts/smoke-rpc.mjs`, `devnet-erc20.mjs`, `build-site.mjs` |
| Foundry sample | `examples/foundry/` — [Quick start](../ops/quickstart.md) · [Recipes](../ops/recipes.md) |
| Hardhat sample | `examples/hardhat/` — same Token/Guestbook · [Quick start §3b](../ops/quickstart.md#3b-hardhat) |
| Guestbook SPA | `examples/guestbook-web/` — [Guestbook product](../product/guestbook.md) · [Try public](../ops/try-public.md) |

### Docs web flow

```mermaid
flowchart TD
  MD["docs/**/*.md + frontmatter"] --> SB[sidebar.yaml]
  MD --> VP[VitePress docs/.vitepress]
  SB --> VP
  VP --> Dev["pnpm docs:dev"]
  VP --> Build["pnpm docs:build"]
  Build --> Dist[docs/.vitepress/dist]
```

Root scripts: `docs:dev` · `docs:build` · `docs:preview` · `site:build` (landing + docs → `dist/`) · `explorer:*` · `faucet:*` · `web:*`.

## Docs categories

| Category | Path | Audience |
| :--- | :--- | :--- |
| Overview … Economics | `docs/overview/` … `docs/economics/` | Protocol readers |
| [Build](./_category.md) | `docs/build/` | Contributors |
| [Networks & ops](../ops/_category.md) | `docs/ops/` | Operators |
| [Product surface](../product/_category.md) | `docs/product/` | Explorer / faucet / Guestbook demo |
| [Scale](../scale/_category.md) | `docs/scale/` | D3 workstreams |
| Security | `docs/security/` | Threat model / audits |

## CI (monorepo)

| Workflow | Trigger | What it runs |
| :--- | :--- | :--- |
| [`ci-go.yml`](../../.github/workflows/ci-go.yml) | PR + push `main` | `go vet`, `go test ./...`, security/load/freeze/chaos gates, build `dew` / `dewcli` / `dewfaucet` |
| [`ci-web.yml`](../../.github/workflows/ci-web.yml) | PR + push `main` | typecheck + `site:build`; explorer + faucet-web builds |
| [`security.yml`](../../.github/workflows/security.yml) | PR + push `main` + weekly | govulncheck, fuzz, pnpm audit, CodeQL, Trivy (HIGH/CRITICAL; SARIF limited to same severities) |
| [`pages.yml`](../../.github/workflows/pages.yml) | push `main` | GitHub Pages deploy of `dist/` |

Heavy multiproc soak (`DEW_HEAVY_INTEGRATION=1`) is not required on every PR.

## What is not multi-repo

| Avoid | Prefer |
| :--- | :--- |
| Separate repos for node / docs / SDK | One monorepo |
| Duplicating markdown for the website | Single `docs/` source |
| Node “light client” that forks protocol rules | Node as RPC consumer only |

## Current status

Monorepo is production-shaped for **public-testnet-v1** (path B live): Go L1 with durable Pebble chaindata, multiproc BFT option, VitePress docs, explorer + faucet + landing SPAs, and deploy samples under `deploy/`.
