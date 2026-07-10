---
title: Addresses
description: Account address derivation and reserved namespaces.
category: protocol
order: 20
status: draft
---

# Addresses

## Format

- 20 bytes, hex-encoded with `0x` prefix (40 hex chars).
- Optional EIP-55 checksum encoding for display.

## Derivation (EOA)

1. Derive uncompressed public key from private key (65 bytes).
2. Drop the `0x04` prefix → 64 bytes.
3. `Keccak-256` → 32 bytes.
4. Take the **last 20 bytes**.

Same as Ethereum — MetaMask-generated keys work unchanged.

## Contract addresses (CREATE)

$$
\text{ContractAddress} = \text{Keccak-256}(\text{RLP}([\text{sender}, \text{nonce}]))[12:32]
$$

`CREATE2` follows the Ethereum formula when the EVM hardfork config enables it (Phase A enables Cancun-era rules from genesis).

## Address namespaces

Dew reserves ranges so dual execution can grow without colliding with user contracts.

| Namespace            | Range                 | Phase | VM / engine                                  |
| :------------------- | :-------------------- | :---- | :------------------------------------------- |
| **EVM space**        | `0x00…00` – `0xdf…ff` | A     | EVM accounts, contracts, precompiles         |
| **Dew-native space** | `0xe0…00` – `0xff…ff` | B     | Native Go modules (optional system programs) |

### Precompile slots (EVM space)

| Range           | Purpose                                       |
| :-------------- | :-------------------------------------------- |
| `0x01` – `0x0a` | Standard Ethereum precompiles                 |
| `0x100`+        | Reserved for Dew system precompiles (Phase B) |

Phase A only requires standard precompiles. Custom Dew precompiles must not be required for basic ERC-20 deploy/transfer.

## Rules

- User EOAs and normal contracts live in EVM space.
- Phase B native modules should use Dew-native space **or** fixed precompile addresses — pick one scheme per module and document it in [Precompiles](../execution/precompiles.md).
- Do not place user-deployable contracts in Dew-native space via normal CREATE without an explicit protocol rule.
