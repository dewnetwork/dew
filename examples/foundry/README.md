# Foundry → Dew

Solidity samples for **local `dew devnet`** and **public-testnet-v1** (chain ID **2205**).

| Doc | Link |
| :--- | :--- |
| 5-minute onboarding | [docs/ops/quickstart.md](../../docs/ops/quickstart.md) |
| Recipes (Token · Guestbook · batch) | [docs/ops/recipes.md](../../docs/ops/recipes.md) |
| Hardhat parity | [examples/hardhat](../hardhat/) |

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

```bash
export DEW_RPC_URL=http://127.0.0.1:8545
export PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
cast chain-id --rpc-url $DEW_RPC_URL   # 2205
```

## Contracts & scripts

| Recipe | Contract | Script(s) |
| :--- | :--- | :--- |
| ERC-20 Token | `src/Token.sol` | `script/Deploy.s.sol` |
| **Guestbook** (flagship) | `src/Guestbook.sol` | `DeployGuestbook.s.sol`, `SignGuestbook.s.sol` |
| Multi-tx batch | (uses Guestbook) | `BatchSignGuestbook.s.sol` |

### Guestbook (recommended demo)

```bash
export MESSAGE="Hello Dew"
forge script script/DeployGuestbook.s.sol:DeployGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
export GUESTBOOK=0x…   # from logs

export MESSAGE="second visit"
forge script script/SignGuestbook.s.sol:SignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv

cast call $GUESTBOOK "totalEntries()(uint256)" --rpc-url $DEW_RPC_URL
cast call $GUESTBOOK "getEntry(uint256)(address,uint64,string)" 0 --rpc-url $DEW_RPC_URL
```

Two messages / one broadcast (multi-tx pack on Dew auto-mine):

```bash
forge script script/BatchSignGuestbook.s.sol:BatchSignGuestbook \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

### Token

```bash
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL --broadcast -vvv
```

## Public testnet

1. New wallet + [faucet](https://faucet-dew.fadosoft.com) (not Anvil #0).
2. `export DEW_RPC_URL=https://rpc-dew.fadosoft.com` and your funded `PRIVATE_KEY`.
3. Deploy Guestbook; open `https://explorer-dew.fadosoft.com` for the address/tx.

## MetaMask

| Field | Local | Public |
| :--- | :--- | :--- |
| Network name | Dew Local | Dew public-testnet-v1 |
| RPC URL | `http://127.0.0.1:8545` | `https://rpc-dew.fadosoft.com` |
| Chain ID | `2205` | `2205` |
| Symbol | DEW | DEW |
| Explorer | (optional) | `https://explorer-dew.fadosoft.com` |

## Related

- [Quick start](../../docs/ops/quickstart.md)
- [Recipes](../../docs/ops/recipes.md)
- [Local devnet](../../docs/ops/devnet.md)
- [Public testnet freeze](../../docs/ops/public-testnet.md)
