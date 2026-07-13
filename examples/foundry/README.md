# Foundry → Dew

Minimal ERC-20 sample for **local `dew devnet`** and **public-testnet-v1** (chain ID **2205**).

Full walkthrough: [docs/ops/quickstart.md](../../docs/ops/quickstart.md).

## Prerequisites

- [Foundry](https://book.getfoundry.sh/getting-started/installation) (`forge`, `cast`)
- Dew node: local `dew devnet` **or** public RPC

```bash
# From monorepo root
go build -o bin/dew ./cmd/dew
./bin/dew devnet --http.port 8545
```

## Setup (once)

```bash
cd examples/foundry
forge install foundry-rs/forge-std --no-git
forge build
forge test
```

## Network params

| | Local | Public |
| :--- | :--- | :--- |
| RPC | `http://127.0.0.1:8545` | `https://rpc-dew.fadosoft.com` |
| Chain ID | `2205` | `2205` |
| Symbol | DEW | DEW |
| Deploy key | Anvil #0 (below) | Key funded by [faucet](https://faucet-dew.fadosoft.com) |

**Anvil #0 (local / private only — never as a public faucet key):**

| | |
| :--- | :--- |
| Address | `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` |
| Private key | `0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80` (`forge script` needs `0x`) |

## Deploy (local)

```bash
export DEW_RPC_URL=http://127.0.0.1:8545
export PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

cast chain-id --rpc-url $DEW_RPC_URL   # 2205

forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL \
  --broadcast \
  -vvv
```

Optional transfer on deploy:

```bash
export TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL \
  --broadcast \
  -vvv
```

## Deploy (public testnet)

1. Import a **new** key into MetaMask (not Anvil #0).
2. Get DEW from `https://faucet-dew.fadosoft.com` (captcha).
3. Export the private key only for CLI deploy (or use MetaMask + Remix).

```bash
export DEW_RPC_URL=https://rpc-dew.fadosoft.com
export PRIVATE_KEY=0xYOUR_FUNDED_KEY   # never Anvil on public

forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL \
  --broadcast \
  -vvv
```

Explorer: `https://explorer-dew.fadosoft.com` (paste tx / address).

## Cast helpers

```bash
cast balance 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 --rpc-url $DEW_RPC_URL
cast call $TOKEN "balanceOf(address)(uint256)" $ADDR --rpc-url $DEW_RPC_URL
cast send $TOKEN "transfer(address,uint256)" $TO 1000000000000000000 \
  --rpc-url $DEW_RPC_URL --private-key $PRIVATE_KEY
```

## MetaMask

| Field | Local | Public |
| :--- | :--- | :--- |
| Network name | Dew Local | Dew public-testnet-v1 |
| RPC URL | `http://127.0.0.1:8545` | `https://rpc-dew.fadosoft.com` |
| Chain ID | `2205` | `2205` |
| Symbol | DEW | DEW |
| Explorer | (optional) | `https://explorer-dew.fadosoft.com` |

## Related

- [Quick start (5 minutes)](../../docs/ops/quickstart.md)
- [Local devnet](../../docs/ops/devnet.md)
- [Public testnet freeze](../../docs/ops/public-testnet.md)
- Monorepo ERC-20 smoke: `node scripts/devnet-erc20.mjs http://127.0.0.1:8545`
