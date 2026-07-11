# Dew

High-performance, EVM-compatible Layer 1 blockchain — built **from scratch** in a **monorepo** using **Go + Node.js**.

| Goal | Approach |
| :--- | :--- |
| **Faster** than Ethereum | ~1s blocks, flat state, later parallel execution |
| **More secure** | Dew-BFT instant finality + slashing |
| **Cheaper** | Higher capacity, discounted storage gas, later native micro-fees |
| **ETH-first** | Solidity / MetaMask / Foundry path before Dew-native features |

## Monorepo

This repository is a **single monorepo** for the whole Dew stack:

| Stack | Role |
| :--- | :--- |
| **Go** | Core L1: node (`dew`), CLI (`dewcli`), ops faucet (`dewfaucet`), consensus, P2P, state, EVM bridge, JSON-RPC server |
| **Node.js** | **Build and serve the documentation website** from `docs/`; plus scripts, localnet helpers, SDK/RPC tests, deploy utilities |

Protocol logic lives in Go. Node is for docs web + developer tooling — not a second consensus client.

Layout details: [docs/development/go-project-layout.md](./docs/development/go-project-layout.md).

## Go (L1 core)

Requires **Go 1.22+** (developed on 1.23).

```bash
go test ./...
go build -o bin/dewcli ./cmd/dewcli

# Wallet CLI (Phase A1)
./bin/dewcli wallet create              # passphrase prompt; keystore ~/.dew/keystore
./bin/dewcli wallet list
./bin/dewcli --keystore /tmp/dew-ks wallet create --password=dev-only

# Full node + JSON-RPC (Phase A4)
go build -o bin/dew ./cmd/dew
./bin/dew run --genesis genesis.json --http.addr 127.0.0.1 --http.port 8545
# MetaMask / Foundry: chainId 2205 (0x89d), RPC http://127.0.0.1:8545
node scripts/smoke-rpc.mjs              # eth_chainId smoke check

# Local devnet: 3 BFT validators + P2P mesh + RPC (Phase A7)
./bin/dew init --out genesis.json
./bin/dew devnet --http.port 8545
go test ./devnet/ -count=1              # BFT + ERC-20 over RPC
# Faucet (Anvil #0): 0xf39F… / ac0974… — see docs/development/devnet.md
```

| Package | Role |
| :--- | :--- |
| [`crypto/`](./crypto/) | secp256k1, Keccak-256, address derivation, sign/verify |
| [`crypto/wallet/`](./crypto/wallet/) | Encrypted keystore (Web3 Secret Storage) |
| [`cmd/dewcli/`](./cmd/dewcli/) | Wallet CLI |
| [`db/`](./db/) | KV store interface + in-memory backend |
| [`core/types/`](./core/types/) | Account, Header, Block, EVM tx / receipt |
| [`core/state/`](./core/state/) | Flat state DB + journal / access list (EVM-ready) |
| [`core/vm/`](./core/vm/) | EVM bridge + sequential `Executor` (Cancun) |
| [`config/`](./config/) | Genesis JSON load + alloc commit |
| [`node/`](./node/) | In-process backend (genesis, auto-mine, state) |
| [`rpc/`](./rpc/) | Ethereum JSON-RPC HTTP (`eth_*` / `net_*` / `web3_*`) |
| [`consensus/`](./consensus/) | Dew-BFT engine + local multi-validator cluster (Phase A5) |
| [`p2p/`](./p2p/) | TCP host, handshake, gossip, sync, consensus fan-out (Phase A6) |
| [`devnet/`](./devnet/) | Local 3-validator + RPC network helpers (Phase A7) |
| [`cmd/dew/`](./cmd/dew/) | Full node entrypoint (`run`, `init`, `devnet`) |
| [`faucet/`](./faucet/) · [`cmd/dewfaucet/`](./cmd/dewfaucet/) | Production faucet HTTP service (Phase D2; ops, not consensus) |
| [`genesis.json`](./genesis.json) | Dev genesis (chainId 2205, 3 validators, faucet alloc) |

## Documentation

Full protocol and build docs: [`docs/`](./docs/README.md).

**Docs website (VitePress):**

```bash
pnpm install
pnpm docs:dev      # local preview (default http://localhost:5173)
pnpm docs:build    # static site → docs/.vitepress/dist
pnpm docs:preview  # serve production build
```

**Block explorer (Phase D1, React SPA):**

```bash
pnpm --dir explorer install
pnpm explorer:dev    # http://localhost:4321 — needs RPC (e.g. dew devnet :8545)
pnpm explorer:build  # → explorer/dist
```

See [`explorer/README.md`](./explorer/README.md) and [block explorer design](./docs/development/block-explorer.md).

**Production faucet (Phase D2):**

```bash
go build -o bin/dewfaucet ./cmd/dewfaucet
# Public: allowlist or captcha modes — never Anvil keys
# Local only: -mode dev -allow-anvil-key
```

See [docs/development/faucet.md](./docs/development/faucet.md).

Markdown under `docs/` is the source of truth; Node only builds the site.

## Landing website

Marketing site lives in [`web/`](./web/) (Astro + React + Tailwind + Motion):

```bash
pnpm web:dev       # http://localhost:4321
pnpm web:build     # static site → web/dist
pnpm web:preview
```

## Combined production site

Build landing + docs into one deployable folder (`dist/`):

```bash
pnpm install
pnpm --dir web install
pnpm build         # or pnpm site:build
pnpm site:preview  # http://localhost:4173  →  / landing, /docs docs
```

Layout after merge:

| Path | Source |
| :--- | :--- |
| `/` | Astro landing (`web/dist`) |
| `/docs/` | VitePress docs (`docs/.vitepress/dist`, base `/docs/`) |

### GitHub Pages

CI deploys `dist/` on every push to `main` via [`.github/workflows/pages.yml`](./.github/workflows/pages.yml).

| | |
| :--- | :--- |
| **URL** | https://dewnetwork.github.io/dew/ |
| **Landing** | `/dew/` |
| **Docs** | `/dew/docs/` |

**One-time enable:** repo **Settings → Pages → Source: GitHub Actions**.

Local build matching production base paths:

```bash
SITE_BASE=/dew/ SITE_URL=https://dewnetwork.github.io pnpm site:build
pnpm site:preview
```

Custom domain later: set `SITE_BASE=/` and `SITE_URL=https://your.domain` in the workflow env (and add the CNAME in Pages settings).

**Start here:**

1. [Vision](./docs/overview/vision.md)
2. [Design principles](./docs/overview/design-principles.md)
3. [Roadmap](./docs/development/roadmap.md)
4. [Implementation phases](./docs/development/phases.md)

## Status

Docs site + landing are wired. **Phase A1–A7** done (crypto → EVM → JSON-RPC → Dew-BFT → P2P → devnet). Next: [Phase B1](./docs/development/phases.md) — parallel execution (Dew-PE).

## License

Licensed under the [Apache License, Version 2.0](./LICENSE).

```
Copyright 2026 Dew Network

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

See also [`NOTICE`](./NOTICE).
