---
title: Builder recipes
description: Copy-paste Foundry and Hardhat recipes on Dew chain 2205 — Token, Mock assets, Guestbook, multi-tx batch.
category: ops
order: 36
status: stable
---

# Builder recipes

Short recipes against **local `dew devnet`** or **public-testnet-v1** (chain ID **2205**).

| Toolchain | Root |
| :--- | :--- |
| **Foundry** (default below) | [examples/foundry](../../examples/foundry/) |
| **Hardhat** | [examples/hardhat](../../examples/hardhat/) — same Token / Mock assets / Guestbook |

### Foundry setup (once)

```bash
go build -o bin/dew ./cmd/dew && ./bin/dew devnet --http.port 8545   # terminal 1
cd examples/foundry
forge install foundry-rs/forge-std --no-git   # once
forge install OpenZeppelin/openzeppelin-contracts@v5.2.0 --no-git   # Mock assets
export DEW_RPC_URL=http://127.0.0.1:8545
export PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
```

### Hardhat setup (once)

```bash
cd examples/hardhat && npm install
# local uses --network dewLocal (Anvil #0). Public: PRIVATE_KEY + --network dewPublic
```

Public: faucet-funded key (never Anvil #0).  
Browser-only demo: [Try public testnet](./try-public.md). Full tool path: [Quick start](./quickstart.md).

---

## Recipe 1 — ERC-20 Token

Deploy minimal `Token` (DST) and optional transfer.

**Foundry**

```bash
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL --broadcast -vvv

export TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

**Hardhat**

```bash
npx hardhat run scripts/deploy-token.js --network dewLocal
TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
  npx hardhat run scripts/deploy-token.js --network dewLocal
```

| | Foundry | Hardhat |
| :--- | :--- | :--- |
| Contract | `src/Token.sol` | `contracts/Token.sol` |
| Deploy | `script/Deploy.s.sol` | `scripts/deploy-token.js` |

---

## Recipe 1b — Mock USDT (testnet)

Deploy **MockUSDT** (OpenZeppelin `ERC20` + `Ownable`): name `Tether USD`, symbol `USDT`, **6 decimals**, + owner `mint`. **Not** real Tether — for app / MetaMask testing only.

**Foundry**

```bash
forge script script/DeployUSDT.s.sol:DeployUSDT \
  --rpc-url $DEW_RPC_URL --broadcast -vvv

export TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8
forge script script/DeployUSDT.s.sol:DeployUSDT \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

**Hardhat**

```bash
npx hardhat run scripts/deploy-usdt.js --network dewLocal
TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
  npx hardhat run scripts/deploy-usdt.js --network dewLocal

# public-testnet-v1
export PRIVATE_KEY=0xYOUR_FUNDED_KEY
npx hardhat run scripts/deploy-usdt.js --network dewPublic
```

| | Foundry | Hardhat |
| :--- | :--- | :--- |
| Contract | `src/MockUSDT.sol` | `contracts/MockUSDT.sol` |
| Deploy | `script/DeployUSDT.s.sol` | `scripts/deploy-usdt.js` |

MetaMask custom token: paste contract address, symbol **USDT**, decimals **6**. Explorer known-token list: `PUBLIC_KNOWN_TOKENS=USDT:0x…` at explorer build time.

---

## Recipe 1c — Mock assets basket (testnet)

Deploy the full mock suite in one go: **USDT · USDC · DAI · WETH · WBTC**. All are OpenZeppelin `ERC20` + `Ownable` with owner `mint`. **Not** real assets — for dApp / MetaMask / multi-token testing only.

| Symbol | Name | Decimals | Default supply |
| :--- | :--- | ---: | :--- |
| USDT | Tether USD | 6 | 1_000_000 |
| USDC | USD Coin | 6 | 1_000_000 |
| DAI | Dai Stablecoin | 18 | 1_000_000 |
| WETH | Wrapped Ether | 18 | 10_000 (mintable stand-in; no native wrap) |
| WBTC | Wrapped BTC | 8 | 100 |

Base type: `MockERC20` (custom name/symbol/decimals). Named wrappers fix metadata.

**Foundry**

```bash
forge script script/DeployMockAssets.s.sol:DeployMockAssets \
  --rpc-url $DEW_RPC_URL --broadcast -vvv

export TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8
forge script script/DeployMockAssets.s.sol:DeployMockAssets \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

**Hardhat**

```bash
npx hardhat run scripts/deploy-mock-assets.js --network dewLocal
TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8 \
  npx hardhat run scripts/deploy-mock-assets.js --network dewLocal

export PRIVATE_KEY=0xYOUR_FUNDED_KEY
npx hardhat run scripts/deploy-mock-assets.js --network dewPublic
```

| | Foundry | Hardhat |
| :--- | :--- | :--- |
| Contracts | `src/MockERC20.sol` + `MockUSDT` / `USDC` / `DAI` / `WETH` / `WBTC` | `contracts/` same names |
| Deploy all | `script/DeployMockAssets.s.sol` | `scripts/deploy-mock-assets.js` |
| Deploy USDT only | `script/DeployUSDT.s.sol` | `scripts/deploy-usdt.js` |

Script logs print each address and a ready-to-paste `PUBLIC_KNOWN_TOKENS=USDT:0x…,USDC:0x…,…` line for explorer build time.

---

## Recipe 2 — On-chain Guestbook (flagship demo)

Append-only messages (max 280 bytes). Share the contract address + explorer link.

### Deploy (+ first message)

**Foundry**

```bash
export MESSAGE="Hello Dew — first visitor"
forge script script/DeployGuestbook.s.sol:DeployGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
export GUESTBOOK=0x…   # from script output
```

**Hardhat**

```bash
MESSAGE="Hello Dew — first visitor" \
  npx hardhat run scripts/deploy-guestbook.js --network dewLocal
export GUESTBOOK=0x…   # from logs
```

### Sign another message

**Foundry**

```bash
export MESSAGE="second signature"
forge script script/SignGuestbook.s.sol:SignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

**Hardhat**

```bash
MESSAGE="second signature" \
  npx hardhat run scripts/sign-guestbook.js --network dewLocal
```

### Read with cast

```bash
cast call $GUESTBOOK "totalEntries()(uint256)" --rpc-url $DEW_RPC_URL
cast call $GUESTBOOK "getEntry(uint256)(address,uint64,string)" 0 --rpc-url $DEW_RPC_URL
```

Public explorer: `https://explorer-dew.fadosoft.com/address/<GUESTBOOK>` or `/tx/<hash>` from broadcast.

| | Foundry | Hardhat |
| :--- | :--- | :--- |
| Contract | `src/Guestbook.sol` | `contracts/Guestbook.sol` |
| Scripts | `DeployGuestbook` / `SignGuestbook` | `deploy-guestbook.js` / `sign-guestbook.js` |

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
# Foundry:
forge script script/SignGuestbook.s.sol:SignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
# Hardhat:
# npx hardhat run scripts/sign-guestbook.js --network dewPublic
```

Product page: [Guestbook demo](../product/guestbook.md).

### Web UI (read + MetaMask sign)

```bash
pnpm guestbook:dev
# http://localhost:4323 — Connect wallet → Sign · or read-only Refresh
# After sign: SPA links to explorer /tx/{hash} and /address/{addr}
# Burst ×2 (C1): two consecutive nonces; confirm both MetaMask prompts quickly
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
- [examples/hardhat](../../examples/hardhat/)
- [Local devnet](./devnet.md)
- [Public testnet](./public-testnet.md)
