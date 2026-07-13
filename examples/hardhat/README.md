# Hardhat → Dew

Solidity samples for **local `dew devnet`** and **public-testnet-v1** (chain ID **2205**).

Same contracts as [examples/foundry](../foundry/) (Token + Guestbook). Prefer Foundry if you already use `forge`; use this kit if you prefer Hardhat / ethers.

| Doc | Link |
| :--- | :--- |
| 5-minute onboarding | [docs/ops/quickstart.md](../../docs/ops/quickstart.md) |
| Recipes | [docs/ops/recipes.md](../../docs/ops/recipes.md) |
| Try public (browser) | [docs/ops/try-public.md](../../docs/ops/try-public.md) |

## Prerequisites

- Node **20+**
- Dew node: local `dew devnet` **or** public RPC

```bash
# From monorepo root
go build -o bin/dew ./cmd/dew
./bin/dew devnet --http.port 8545
```

## Setup (once)

```bash
cd examples/hardhat
npm install
npm run compile
npm test
```

Optional: `cp .env.example .env` and edit keys.

## Networks

| Network name | Default RPC | Chain ID | Accounts |
| :--- | :--- | :--- | :--- |
| `dewLocal` | `http://127.0.0.1:8545` | `2205` | Anvil #0 (or `PRIVATE_KEY`) |
| `dewPublic` | `https://rpc-dew.fadosoft.com` | `2205` | **`PRIVATE_KEY` required** (faucet-funded) |
| `hardhat` | in-process | — | built-in (unit tests only) |

Override RPC with `DEW_RPC_URL`.

**Anvil #0 (local / private only — never public):**

| | |
| :--- | :--- |
| Address | `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` |
| Private key | `0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80` |

## Deploy Token

```bash
npx hardhat run scripts/deploy-token.js --network dewLocal

# optional transfer 1000 DST
TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
  npx hardhat run scripts/deploy-token.js --network dewLocal
```

## Guestbook

```bash
MESSAGE="Hello Dew" npx hardhat run scripts/deploy-guestbook.js --network dewLocal
# copy logged Guestbook address

export GUESTBOOK=0x…
MESSAGE="second visit" npx hardhat run scripts/sign-guestbook.js --network dewLocal
```

Public path B (live contract — no redeploy required):

```bash
export PRIVATE_KEY=0xYOUR_FAUCET_FUNDED_KEY
export GUESTBOOK=0x83bB4E539BE46503481E66094b01b854990BF84a
MESSAGE="hello from hardhat" \
  npx hardhat run scripts/sign-guestbook.js --network dewPublic
```

Explorer: `https://explorer-dew.fadosoft.com/address/<GUESTBOOK>` · `/tx/<hash>`.

## Public testnet

1. New wallet + [faucet](https://faucet-dew.fadosoft.com) (not Anvil #0).
2. `export PRIVATE_KEY=0x…` (and optional `DEW_RPC_URL`).
3. `--network dewPublic` for deploy/sign scripts.

## npm scripts

| Script | Command |
| :--- | :--- |
| `npm run compile` | `hardhat compile` |
| `npm test` | in-process Hardhat network |
| `npm run deploy:token` | needs `--network` via hardhat CLI |
| `npm run deploy:guestbook` | same |
| `npm run sign:guestbook` | same |

Prefer explicit:

```bash
npx hardhat run scripts/deploy-token.js --network dewLocal
```

## Related

- [examples/foundry](../foundry/) — forge parity
- [Quick start](../../docs/ops/quickstart.md)
- [Recipes](../../docs/ops/recipes.md)
- [Public testnet freeze](../../docs/ops/public-testnet.md)
