# Dew Guestbook (read UI)

Read-only SPA for the on-chain [Guestbook](../foundry/src/Guestbook.sol) on **public-testnet-v1** (chain **2205**).

Default contract (public deploy):

```text
0x83bB4E539BE46503481E66094b01b854990BF84a
```

RPC: `https://rpc-dew.fadosoft.com`

## Run

```bash
cd examples/guestbook-web
pnpm install
pnpm dev          # http://localhost:4323
```

```bash
pnpm build
pnpm preview
```

From monorepo root:

```bash
pnpm guestbook:dev
pnpm guestbook:build
```

## Env (optional, bake at build)

| Variable | Default |
| :--- | :--- |
| `PUBLIC_RPC_URL` | `https://rpc-dew.fadosoft.com` |
| `PUBLIC_GUESTBOOK` | `0x83bB4E…F84a` (public deploy) |
| `PUBLIC_EXPLORER_URL` | `https://explorer-dew.fadosoft.com` |
| `PUBLIC_CHAIN_ID` | `2205` |

UI also lets you override RPC / address at runtime (local `dew devnet` + your deploy).

## Sign new messages

This app **only reads**. To post:

```bash
cd examples/foundry
export DEW_RPC_URL=https://rpc-dew.fadosoft.com
export PRIVATE_KEY=0x…
export GUESTBOOK=0x83bB4E539BE46503481E66094b01b854990BF84a
export MESSAGE="from the web reader"
forge script script/SignGuestbook.s.sol:SignGuestbook \
  --rpc-url "$DEW_RPC_URL" --broadcast -vvv
```

Then hit **Refresh** in the UI.

Recipes: [docs/ops/recipes.md](../../docs/ops/recipes.md).
