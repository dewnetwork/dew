---
title: Ethereum Compatibility
description: What is compatible in Phase A, what is deferred, and what deliberately differs.
category: overview
order: 30
status: draft
---

# Ethereum Compatibility

## Intent

Dew should feel like a **custom EVM chain** to developers: same keys, same contracts, same RPC habits. Differences appear in consensus, finality, fees level, and (later) optional native APIs.

## Compatible in Phase A

| Area             | Compatibility                                                       |
| :--------------- | :------------------------------------------------------------------ |
| Keys & addresses | secp256k1, Keccak-256, 20-byte `0x` addresses                       |
| Account model    | Nonce, balance, code, storage                                       |
| Smart contracts  | Solidity / Vyper → EVM bytecode                                     |
| Tx types         | EIP-1559 (type `0x02`); legacy support optional but recommended     |
| Signing          | ECDSA; EIP-155 / EIP-1559 chain ID rules                            |
| JSON-RPC         | Core `eth_*`, `net_*`, `web3_*` for wallets and deploy tools        |
| Tooling          | MetaMask custom network, Hardhat, Foundry, ethers, viem, web3.js    |
| Gas model        | Gas units + EIP-1559 base fee / tip (gas _schedule_ may be cheaper) |

## Deliberately different (still “EVM compatible”)

| Area          | Ethereum         | Dew                                 |
| :------------ | :--------------- | :--------------------------------------- |
| Consensus     | Gasper (PoS)     | Dew-BFT (PoSA / bonded validators)       |
| Finality      | Epoch-based      | Instant on commit                        |
| Block time    | ~12s target      | ~1s target (_tentative_)                 |
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

MetaMask: add custom network with the RPC URL above; block explorer URL = explorer base only. Full publish template: [Launch checklist](../development/launch-checklist.md).

## Deferred to Phase B (not required for first devnet)

- `DewTx` binary transactions and `dew_sendRawTransaction`
- Parallel execution engine (Dew-PE / Block-STM)
- Custom precompiles (native DEX, staking bridge from Solidity)
- Dew address namespace for native modules (`0xe0…`–`0xff…`)

## Minimum RPC for “MetaMask + Foundry works”

Must work before claiming Phase A complete:

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

Detailed vectors will live under monorepo `tests/` (Go) and optional Node RPC smoke scripts as phases land.
