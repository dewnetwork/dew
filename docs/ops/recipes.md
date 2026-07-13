---
title: Builder recipes
description: Copy-paste Foundry recipes on Dew chain 2205 — Token, Guestbook, multi-tx batch.
category: ops
order: 36
status: stable
---

# Builder recipes

Three short recipes against **local `dew devnet`** or **public-testnet-v1** (chain ID **2205**). Project root: [examples/foundry](../../examples/foundry/).

Setup once:

```bash
go build -o bin/dew ./cmd/dew && ./bin/dew devnet --http.port 8545   # terminal 1
cd examples/foundry
forge install foundry-rs/forge-std --no-git   # once
export DEW_RPC_URL=http://127.0.0.1:8545
export PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
```

Public: set `DEW_RPC_URL=https://rpc-dew.fadosoft.com` and a **faucet-funded** key (never Anvil #0).  
Browser-only demo: [Try public testnet](./try-public.md). Full tool path: [Quick start](./quickstart.md).

---

## Recipe 1 — ERC-20 Token

Deploy minimal `Token` (DST) and optional transfer.

```bash
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL --broadcast -vvv

# optional transfer on deploy
export TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

| Contract | `src/Token.sol` |
| Script | `script/Deploy.s.sol` |

---

## Recipe 2 — On-chain Guestbook (flagship demo)

Append-only messages (max 280 bytes). Share the contract address + explorer link.

### Deploy (+ first message)

```bash
export MESSAGE="Hello Dew — first visitor"
forge script script/DeployGuestbook.s.sol:DeployGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
# copy logged Guestbook address → GUESTBOOK
export GUESTBOOK=0x…   # from script output
```

### Sign another message

```bash
export MESSAGE="second signature"
forge script script/SignGuestbook.s.sol:SignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

### Read with cast

```bash
cast call $GUESTBOOK "totalEntries()(uint256)" --rpc-url $DEW_RPC_URL
cast call $GUESTBOOK "getEntry(uint256)(address,uint64,string)" 0 --rpc-url $DEW_RPC_URL
```

Public explorer: `https://explorer-dew.fadosoft.com/address/<GUESTBOOK>` or `/tx/<hash>` from broadcast.

| Contract | `src/Guestbook.sol` |
| Scripts | `DeployGuestbook.s.sol`, `SignGuestbook.s.sol` |

**Why this demo:** one contract, faucet DEW, MetaMask-friendly, visible on explorer — good “I used Dew” story without bridges or staking.

### Public path B defaults

| Item | Value |
| :--- | :--- |
| SPA | `https://guestbook-dew.fadosoft.com` |
| Contract | `0x83bB4E539BE46503481E66094b01b854990BF84a` |

```bash
# Use the live contract without redeploying
export GUESTBOOK=0x83bB4E539BE46503481E66094b01b854990BF84a
export MESSAGE="hello from recipes"
forge script script/SignGuestbook.s.sol:SignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

Product page: [Guestbook demo](../product/guestbook.md).

### Web UI (read + MetaMask sign)

```bash
pnpm guestbook:dev
# http://localhost:4323 — Connect wallet → Sign · or read-only Refresh
# After sign: SPA links to explorer /tx/{hash} and /address/{addr}
```

Public host: `https://guestbook-dew.fadosoft.com`  
Packaging: [deploy/guestbook](../../deploy/guestbook/) · full stack edge in [deploy/docker-compose.yml](../../deploy/docker-compose.yml).  
SPA: [examples/guestbook-web](../../examples/guestbook-web/).

---

## Recipe 3 — Multi-tx batch (C1 pack)

Showcase **C1**: continuous nonces from one sender are packed into a single block (up to **`node.DefaultMaxTxsPerBlock = 64`**). Future nonces with a gap stay pending until the gap fills.

Post **two** guestbook messages in one forge script (two nonces). With Dew **auto-mine**, ready nonces pack together.

```bash
# Local: deploy first (Recipe 2). Public: may use live GUESTBOOK below.
export GUESTBOOK=${GUESTBOOK:-0x83bB4E539BE46503481E66094b01b854990BF84a}
export MESSAGE_A="batch line A"
export MESSAGE_B="batch line B"
forge script script/BatchSignGuestbook.s.sol:BatchSignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

Check that both entries exist:

```bash
cast call $GUESTBOOK "totalEntries()(uint256)" --rpc-url $DEW_RPC_URL
```

On the explorer, open each tx hash; under auto-mine they often share the same block number when nonces were contiguous.

| Script | `script/BatchSignGuestbook.s.sol` |
| Protocol | [C1 multi-tx](../build/phases.md#c1--mempool-admission--fee-policy) · `node.DefaultMaxTxsPerBlock` |

**Expect:** continuous nonces → multi-tx block; nonce gap → later txs wait in mempool (not dropped solely for being “future”). DewTx auto-mine multi-pack is still optional residual ([agents/debt.md](../../agents/debt.md)).

---

## MetaMask (any recipe)

| Field | Local | Public |
| :--- | :--- | :--- |
| RPC | `http://127.0.0.1:8545` | `https://rpc-dew.fadosoft.com` |
| Chain ID | `2205` | `2205` |
| Symbol | DEW | DEW |
| Explorer | — | `https://explorer-dew.fadosoft.com` |

Import Anvil #0 **only** on local nets. Public: new wallet + [faucet](https://faucet-dew.fadosoft.com).

---

## Related

- [Try public testnet](./try-public.md)
- [Quick start (5 minutes)](./quickstart.md)
- [Guestbook product](../product/guestbook.md)
- [examples/foundry](../../examples/foundry/)
- [Local devnet](./devnet.md)
- [Public testnet](./public-testnet.md)
