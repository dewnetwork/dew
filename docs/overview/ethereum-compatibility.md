---
title: Ethereum Compatibility
description: What is compatible in Phase A, what is deferred, and what deliberately differs.
category: overview
order: 30
status: stable
---

# Ethereum Compatibility

## Intent

Dew should feel like a **custom EVM chain** to developers: same keys, same contracts, same RPC habits. Differences appear in consensus, finality, fees level, and (later) optional native APIs.

## Compatible today (public-testnet-v1)

| Area | Compatibility |
| :--- | :--- |
| Keys & addresses | secp256k1, Keccak-256, 20-byte `0x` addresses |
| Account model | Nonce, balance, code, storage |
| Smart contracts | Solidity / Vyper → EVM bytecode |
| Tx types | EIP-1559 (type `0x02`) and legacy type 0 |
| Signing | ECDSA; EIP-155 / EIP-1559 chain ID rules |
| JSON-RPC | Core `eth_*`, `net_*`, `web3_*` for wallets and deploy tools |
| Tooling | MetaMask custom network, Hardhat, Foundry, ethers, viem, web3.js |
| Gas model | Gas units + EIP-1559 base fee / tip |

## Deliberately different (still “EVM compatible”)

| Area          | Ethereum         | Dew                                 |
| :------------ | :--------------- | :--------------------------------------- |
| Consensus     | Gasper (PoS)     | Dew-BFT (PoSA / bonded validators)       |
| Finality      | Epoch-based      | Instant on commit                        |
| Block time    | ~12s target      | ~1s multiproc BFT pace (Path B: auto-mine on admit) |
| State backend | MPT primary path | Flat DB + SMT commitment                 |
| Chain ID      | 1 (mainnet)      | `2205` public-testnet-v1 (frozen C6)     |
| Token         | ETH              | DEW (18 decimals)                        |

“EVM compatible” means **contracts and tooling**, not identical consensus or economics.

### Public testnet (live)

| Field | Value |
| :---- | :---- |
| Network | Dew public-testnet-v1 |
| Chain ID | `2205` |
| RPC URL | `https://rpc-dew.fadosoft.com` |
| Symbol | DEW |
| Block explorer | `https://explorer-dew.fadosoft.com` |
| Faucet | `https://faucet-dew.fadosoft.com` |
| Guestbook SPA | `https://guestbook-dew.fadosoft.com` |
| Guestbook contract | `0x83bB4E539BE46503481E66094b01b854990BF84a` |
| Mock USDT | `0x43d08b71B0A5626a9D0eB607C2aE5277255cFbC8` (6 dec, test only) |
| Mock USDC | `0xacDd0BF26C96879b4c4CeE8F92928f1760F07119` (6 dec) |
| Mock DAI | `0x9813B1738eb2982F3D0C1E22F9C7c4F07B670a1B` (18 dec) |
| Mock WETH | `0x5b88b2ab38920197328f9D90BaAf29cb91F7E3CB` (18 dec) |
| Mock WBTC | `0xdB6E09cb954Ae645C815Bc941bDfD85D4144166E` (8 dec) |

MetaMask: add custom network with the RPC URL above; block explorer URL = explorer base only. Import mock tokens by address (or use explorer known-token balances). Full publish template: [Launch checklist](../ops/launch-checklist.md). Browser-only path: [Try public testnet](../ops/try-public.md). Live mock table: [Public testnet freeze](../ops/public-testnet.md#live-mock-erc-20-basket-path-b).

## Dew-specific (optional; does not block ETH tooling)

| Surface | Status |
| :--- | :--- |
| `DewTx` + `dew_sendRawTransaction` | Shipped; default native path on |
| Dew-PE parallel execution | Shipped (fork+overlay; serial-equivalent) |
| Precompiles `0x100` / `0x102` | Shipped; staking default **off** |
| Mixed DewTx in EVM block body | Not under public-testnet-v1 (index/receipt path) |
| Dew address namespace `0xe0…`–`0xff…` | Reserved for future modules |

## Minimum RPC for “MetaMask + Foundry works”

Required surface (shipped):

- `eth_chainId`, `net_version`, `web3_clientVersion`
- `eth_blockNumber`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getCode`
- `eth_sendRawTransaction`, `eth_call`, `eth_estimateGas`
- `eth_getTransactionReceipt`, `eth_getBlockByNumber`, `eth_getBlockByHash`
- `eth_gasPrice` and/or fee fields consistent with EIP-1559
- `eth_getLogs` (or filters) for basic indexing

See [JSON-RPC](../api/json-rpc.md) for the full matrix.

## Compatibility test plan (summary)

1. Import account into MetaMask → show DEW balance
2. Foundry / Hardhat deploy ERC-20
3. Transfer tokens; receipt status `0x1`
4. `eth_call` view methods match on-chain state
5. Replay known RLP fixtures against local node

**Copy-paste paths:**

- Browser: [Try public testnet](../ops/try-public.md) (faucet → MetaMask → [Guestbook](../product/guestbook.md) → explorer)
- Local + Foundry: [Quick start](../ops/quickstart.md) · [examples/foundry](../../examples/foundry/)
- Local + Hardhat: [Quick start §3b](../ops/quickstart.md#3b-hardhat) · [examples/hardhat](../../examples/hardhat/)
- Recipes (Guestbook, multi-tx C1): [Builder recipes](../ops/recipes.md)

Detailed vectors live under monorepo `tests/` / `devnet/` (Go) and Node scripts (`scripts/smoke-rpc.mjs`, `scripts/devnet-erc20.mjs`).
