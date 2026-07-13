---
title: Quick start (5 minutes)
description: Local dew devnet, MetaMask, and Foundry deploy on chain ID 2205.
category: ops
order: 35
status: stable
---

# Quick start (5 minutes)

Ship a Solidity ERC-20 against **Dew** — local first, then optional public testnet. Chain ID is always **`2205`**.

| Step | Local | Public (optional) |
| :--- | :--- | :--- |
| 1. Node | `dew devnet` | already live |
| 2. RPC | `http://127.0.0.1:8545` | `https://rpc-dew.fadosoft.com` |
| 3. Funds | Anvil #0 pre-funded | [Faucet](https://faucet-dew.fadosoft.com) (captcha) |
| 4. Deploy | [examples/foundry](../../examples/foundry/) | same project, different key + RPC |

Freeze tag: **`public-testnet-v1`**. Full operator surface: [Public testnet](./public-testnet.md).

---

## 0. Prerequisites

| Tool | Why |
| :--- | :--- |
| **Go 1.25+** | Build `dew` (see root `go.mod`) |
| **Foundry** (`forge`, `cast`) | [Install](https://book.getfoundry.sh/getting-started/installation) |
| **Node 20+** (optional) | `scripts/smoke-rpc.mjs` |

```bash
# Foundry (once)
curl -L https://foundry.paradigm.xyz | bash
foundryup
forge --version
```

---

## 1. Start local Dew (≈1 min)

From the **monorepo root**:

```bash
go build -o bin/dew ./cmd/dew
./bin/dew devnet --http.port 8545
```

Leave this terminal running. Smoke in another shell:

```bash
node scripts/smoke-rpc.mjs http://127.0.0.1:8545
# expect eth_chainId 0x89d (2205)
```

| Item | Value |
| :--- | :--- |
| Chain ID | `2205` (`0x89d`) |
| JSON-RPC | `http://127.0.0.1:8545` |
| Block production | Auto-mine packs ready pending EVM txs (nonce chains, up to 64 / block) |
| Details | [Local devnet](./devnet.md) |

---

## 2. MetaMask (local)

1. **Settings → Networks → Add network**
2. Fill:

| Field | Value |
| :--- | :--- |
| Network name | Dew Local |
| RPC URL | `http://127.0.0.1:8545` |
| Chain ID | `2205` |
| Currency symbol | DEW |

3. Import account with Anvil #0 private key (**local only**):

```text
ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
```

Address: `0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266` — balance should be **1,000,000 DEW**.

**Never** use Anvil keys on the public faucet, as a public validator, or in a public deploy tutorial that targets mainnet-like endpoints with long-lived value.

---

## 3. Foundry: build, test, deploy (≈3 min)

```bash
cd examples/foundry
forge install foundry-rs/forge-std --no-git
forge build
forge test
```

Deploy Token (1,000,000 DST to the deployer):

```bash
export DEW_RPC_URL=http://127.0.0.1:8545
# forge script requires 0x-prefixed hex for vm.envUint
export PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

cast chain-id --rpc-url $DEW_RPC_URL   # 2205

forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL \
  --broadcast \
  -vvv
```

Optional: transfer `1000` DST to Anvil #1 on the same script run:

```bash
export TRANSFER_TO=0x70997970C51812dc3A010C7d01b50e0d17dc79C8
forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL \
  --broadcast \
  -vvv
```

Copy the logged `Token` address, then:

```bash
export TOKEN=0x…   # from script output
cast call $TOKEN "balanceOf(address)(uint256)" \
  0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 \
  --rpc-url $DEW_RPC_URL
```

Project README: [examples/foundry/README.md](../../examples/foundry/README.md).

**Next recipes** (Guestbook + multi-tx batch): [Builder recipes](./recipes.md).

Without Foundry, the monorepo also has:

```bash
node scripts/devnet-erc20.mjs http://127.0.0.1:8545
go test ./devnet/ -run ERC20 -v
```

---

## 4. Public testnet (optional)

Path B is live (single-host controlled RPC). Same chain ID and Foundry project.

| Surface | URL |
| :--- | :--- |
| JSON-RPC | `https://rpc-dew.fadosoft.com` |
| Explorer | `https://explorer-dew.fadosoft.com` |
| Faucet | `https://faucet-dew.fadosoft.com` |

```bash
node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com
```

1. Create a **new** wallet (not Anvil #0).
2. MetaMask: RPC `https://rpc-dew.fadosoft.com`, chain ID **2205**, symbol **DEW**, explorer base `https://explorer-dew.fadosoft.com` (no path suffix).
3. Request DEW from the faucet (captcha · 1 DEW / address / 24h).
4. Deploy:

```bash
export DEW_RPC_URL=https://rpc-dew.fadosoft.com
export PRIVATE_KEY=0xYOUR_FUNDED_KEY

forge script script/Deploy.s.sol:Deploy \
  --rpc-url $DEW_RPC_URL \
  --broadcast \
  -vvv
```

5. Open the tx or contract address on the explorer.

Publish template and ops: [Launch checklist](./launch-checklist.md) · [Public testnet freeze](./public-testnet.md).

---

## 5. Troubleshooting

| Symptom | Check |
| :--- | :--- |
| `eth_chainId` ≠ `0x89d` | Wrong RPC or wrong network |
| `forge script` connection refused | Is `dew devnet` listening on `:8545`? |
| Insufficient funds (public) | Faucet drip; gas is small but non-zero |
| MetaMask “nonce too high” | Reset account (local re-genesis) or wait for public tip |
| Want full BFT mesh docs | [Local devnet](./devnet.md) · multiproc: [Private testnet](./private-testnet.md) |

---

## Related

- [Builder recipes](./recipes.md) — Token, **Guestbook** demo, multi-tx batch
- [examples/foundry](../../examples/foundry/) — contracts + forge scripts
- [Local devnet](./devnet.md) — faucet keys, topology, ERC-20 smoke
- [Ethereum compatibility](../overview/ethereum-compatibility.md)
- [JSON-RPC](../api/json-rpc.md)
- [Block explorer](../product/block-explorer.md)
