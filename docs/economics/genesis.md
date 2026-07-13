---
title: Genesis
description: Genesis block schema, alloc, and initial validators.
category: economics
order: 20
status: stable
---

# Genesis

The genesis file bootstraps chain identity, fork flags, pre-funded accounts, and the first validator set.

## Example `genesis.json`

```json
{
  "config": {
    "chainId": 2205,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "muirGlacierBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "shanghaiBlock": 0,
    "cancunBlock": 0,
    "consensus": {
      "type": "dew-bft",
      "epochLength": 86400,
      "unbondingPeriodSeconds": 604800,
      "minValidatorStake": "100000000000000000000000",
      "activeValidatorCap": 21
    }
  },
  "nonce": "0x0",
  "timestamp": 0,
  "extraData": "0x446577636861696e",
  "gasLimit": "0x7270e00",
  "difficulty": "0x1",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "coinbase": "0x0000000000000000000000000000000000000000",
  "baseFeePerGas": "0x3b9aca00",
  "alloc": {},
  "initialValidators": []
}
```

### Notes

- Hardfork blocks at `0` → Cancun-era EVM rules from genesis.
- `unbondingPeriodSeconds` uses **seconds** (not blocks) in config naming to avoid ambiguity.
- `gasLimit` `0x7270e00` = 120,000,000.
- `chainId` **2205** is frozen for **public-testnet-v1** (`params.PublicTestnetChainID`). See [Public testnet freeze](../ops/public-testnet.md).
- Set a real `timestamp` at launch; `0` is a placeholder for local dev.
- Sample `activeValidatorCap` may be lower than module default \(K=100\) for small local nets.

## `alloc`

Map of address → `{ "balance": "wei", "code": "0x...", "storage": {...} }`.

Genesis supply should sum to the intended 1B DEW (or documented exception for testnets).

## `initialValidators`

```json
{
  "address": "0x...",
  "pubKey": "0x04...",
  "votingPower": 100000
}
```

Minimum **3** validators recommended for local BFT testing.

## Init and run (operator UX)

```bash
# Write sample genesis (3 Anvil-compatible validators + faucet alloc)
dew init --out genesis.json

# Devnet (in-process BFT + RPC)
dew devnet --http.port 8545

# Durable single/multi node
dew run --genesis genesis.json --datadir ./data \
  --http.port 8545
```

Effects of first open with genesis:

1. Write genesis block (height 0) into `<datadir>/chaindata`
2. Apply `alloc` to flat state
3. Persist tip + indexes
4. Ready for JSON-RPC and optional `--validator` BFT

See [Durable chaindata](../ops/durable-chaindata.md), [Local Devnet](../ops/devnet.md).
