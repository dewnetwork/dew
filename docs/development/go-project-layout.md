---
title: Monorepo Layout
description: Dewchain monorepo — Go L1 core plus Node.js for docs website and tooling.
category: development
order: 10
status: draft
---

# Monorepo Layout

Dewchain is developed as a **single monorepo**: one git repository, one product surface, shared docs and CI.

## Stack split

| Language | Responsibility | Package manager |
| :--- | :--- | :--- |
| **Go** | L1 node, consensus, P2P, state, EVM, RPC server, `dewcli` | `go.mod` / `go.sum` at repo root |
| **Node.js** | **Documentation website** (build & preview from `docs/`); scripts, localnet, SDK/RPC tests, deploy helpers | `package.json` / lockfile at repo root |

### Why Node is in the monorepo

1. **Web docs (primary DX for documentation)** — Markdown in `docs/` is authored for humans and for a static site generator. Building that site (dev server, production static export) is a **Node** job, not a Go job.
2. **Tooling later** — localnet orchestration, RPC smoke tests, optional TS SDK.

### Rules of ownership

- **Canonical chain logic is Go.** Validators and full nodes run Go binaries only.
- **Node does not reimplement consensus or state transition.** It consumes the Go node via JSON-RPC / process orchestration when used for tooling.
- **Docs source is `docs/`.** Node only *builds and serves* the website from that tree; it does not own protocol truth.
- Prefer **one root** for each stack (`go.mod`, `package.json`) unless a future workspace split is explicitly justified.
- Shared constants that both sides need (chain ID, ports) should be documented in genesis/docs and, when generated, emitted from a single source (e.g. Go `params` or a small JSON consumed by both).

## Target tree

```
dewchain/                          # monorepo root
├── cmd/
│   ├── dewchain/                  # Go: full node entrypoint
│   └── dewcli/                    # Go: wallet / util CLI
├── core/
│   ├── types/                     # Header, block, tx, receipt
│   ├── state/                     # Flat state, journal, roots
│   └── vm/                        # EVM wrapper + StateDB bridge
├── crypto/                        # secp256k1, keccak, keystore helpers
├── db/                            # KV backend abstraction
├── consensus/                     # Dew-BFT
├── p2p/                           # Networking
├── rpc/                           # JSON-RPC server
├── config/                        # Genesis + node config
├── params/                        # Chain constants, gas tables
├── tests/                         # Go integration / multi-node tests
├── scripts/                       # Shell + Node helper scripts
├── packages/                      # Node workspaces (optional as we grow)
│   ├── sdk/                       # TS client for eth_* / dew_* (later)
│   └── localnet/                  # Spin up multi-validator devnet (later)
├── docs/                          # Protocol + engineering docs (markdown source)
│   ├── **/*.md                    # Pages + YAML frontmatter for the docs site
│   └── sidebar.yaml               # Sidebar map for the site generator
├── go.mod                         # Go module root
├── go.sum
├── package.json                   # Node root — docs site build + tooling
├── package-lock.json              # or pnpm-lock.yaml / yarn.lock
└── README.md
```

Early phases may only have `docs/`, root `package.json`, and the first Go packages — the tree above is the **target monorepo shape**, not a claim that every folder exists today.

## Go packages

Layout follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout) adapted for a blockchain node.

| Rule                       | Why                                                   |
| :------------------------- | :---------------------------------------------------- |
| `cmd/*` is thin            | Business logic in libraries                           |
| No import cycles           | `consensus` → `core` ok; use interfaces at boundaries |
| `params` has no heavy deps | Constants only                                        |
| Codecs versioned           | Avoid silent fork of encodings                        |

- Go **1.22+** (_tentative_)
- Unit tests next to packages; multi-node tests under `tests/`
- `golangci-lint` for static analysis

## Node packages

Node is the **docs website + tooling lane** of the monorepo:

| Use | Priority | Examples |
| :--- | :--- | :--- |
| **Docs website** | **Primary Node role** | Dev server + production build from `docs/**/*.md` (VitePress / Nextra / Docusaurus / etc.) |
| Scripts | As needed | `localnet`, genesis helpers, faucet |
| Tests | As needed | RPC smoke tests against a running Go node |
| SDK | Later | TypeScript client for `eth_*` / `dew_*` |

### Docs web flow

```
docs/**/*.md  (+ frontmatter, sidebar.yaml)
        │
        ▼
  Node docs site generator
        │
        ├── docs:dev   → local preview
        └── docs:build → static site for hosting
```

- Authors edit markdown only under `docs/`.
- Site theme/config will live with the Node app when added; **do not fork content** into a second copy.
- Generator choice is not frozen; frontmatter (`title`, `description`, `category`, `order`, `status`) stays portable.

Suggested root script names (wire when the site is set up — docs only for now):

```json
{
  "scripts": {
    "docs:dev": "/* preview docs website */",
    "docs:build": "/* production static build */",
    "test:rpc": "/* optional RPC smoke */",
    "localnet": "/* optional orchestrate Go binaries */"
  }
}
```

## CI (monorepo)

Typical pipeline lanes:

1. **Go**: `go test ./...`, lint, build `dewchain` / `dewcli`
2. **Node / docs**: install deps, `docs:build` (markdown site still compiles)
3. **Integration** (later): start Go node → run Node RPC smoke tests

Failing either stack fails the monorepo build for that PR when that stack is in use.

## What is not multi-repo

| Avoid (for now) | Prefer |
| :--- | :--- |
| Separate repos for node / docs site / SDK | One monorepo |
| Duplicating markdown for the website | Single `docs/` source; Node only builds it |
| Duplicate genesis constants in three places | One genesis schema + generated clients if needed |
| Node “light client” that forks protocol rules | Node as RPC consumer only (for chain tooling) |

## Current status

Monorepo established: markdown docs organized for a future **Node-built docs website**, plus root `package.json`. Go module and packages land with Phase A. Wire `docs:dev` / `docs:build` when choosing a site generator — no requirement to implement that in the same change as content edits.
