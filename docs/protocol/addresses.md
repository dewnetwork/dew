---
title: Addresses
description: Account address derivation and reserved namespaces.
category: protocol
order: 20
status: stable
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

`CREATE2` follows the Ethereum formula under Cancun-era rules enabled from genesis.

## Address namespaces

Dew reserves ranges so dual execution can grow without colliding with user contracts.

| Namespace | Range | Use |
| :--- | :--- | :--- |
| **EVM space** | `0x00…00` – `0xdf…ff` | EVM accounts, contracts, precompiles |
| **Dew-native space** | `0xe0…00` – `0xff…ff` | Reserved for future native modules |

### Precompile slots (EVM space)

Fixed **low addresses** inside EVM space, callable like contracts but implemented in Go when registered. Layout, gas, and call conventions: [Precompiles](../execution/precompiles.md). Code registry: `vm.DewPrecompileSlots()` · freeze lows: `params.Precompile*Addr`.

#### Ethereum standard (Cancun)

| Range | Purpose | Status (public-testnet-v1) |
| :--- | :--- | :--- |
| `0x01` – `0x0a` | Standard Ethereum precompiles (Cancun set) | **Active** (geth Cancun set via executor) |

#### Dew system range (`0x100+`)

| Address | Name | Status | Live map | Gas |
| :--- | :--- | :--- | :--- | :--- |
| `0x100` | Native transfer | **Active** when Dew precompiles on | Yes | 3_000 fixed (`params.NativeTransferPrecompileGas`) |
| `0x101` | Native swap / orderbook | **Reserved** | **No** (empty account / fail-closed) | None (TBD only if activated later) |
| `0x102` | Staking entrypoint | **Flagged** — methods need staking on (default **off**) | Yes | Method-based (`params/staking.go`) |
| `0x103+` | Unallocated | Free for future assignment | — | — |

**Next free slot:** `0x103` (`params.PrecompileNextFreeAddr` / `vm.NextFreeDewPrecompileSlot`).

Custom Dew precompiles must not be required for basic ERC-20 deploy/transfer.

#### Allocation rules

1. **Assign the next free low address** (`0x103`, then `0x104`, …). Do not skip arbitrarily without documenting why.
2. **Never reuse a retired slot** for a different module — retire permanently or leave as empty forever.
3. **Reserved → active** (e.g. shipping a live `0x101` swap) requires a **hardfork doc** under freeze `public-testnet-v1`; prefer config flags only for method gating on already-live addresses (as with `0x102` staking).
4. **Gas schedules** exist only for **active / flagged-live** methods. Reserved slots must not invent a live gas table until activation.
5. **Feature flags** may disable the whole Dew map (`EnableDewPrecompiles`) or gate methods (`EnableStaking`); they do not free a slot for reuse by another module.

Cross-link: freeze table in [Public testnet](../ops/public-testnet.md#freeze-table).

#### EVM precompile slots vs Dew-native space

| Scheme | When to use |
| :--- | :--- |
| **EVM precompile slot** (`0x01–0x0a`, `0x100+`) | Solidity/CALL-visible system contract; ETH tooling treats it as an address |
| **Dew-native space** (`0xe0…` – `0xff…`) | Future native modules that must **not** collide with EVM CREATE / user contracts; not exposed as EVM precompiles unless a separate design promotes them |

Pick **one** scheme per module. Do not place user-deployable contracts in Dew-native space via normal CREATE without an explicit protocol rule.

## Rules

- User EOAs and normal contracts live in EVM space.
- System modules use fixed precompile addresses **or** documented native-space rules — pick one scheme per module.
- Do not place user-deployable contracts in Dew-native space via normal CREATE without an explicit protocol rule.
