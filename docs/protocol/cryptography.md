---
title: Cryptography
description: Key pairs, hashing, and signature standards for Dew.
category: protocol
order: 10
status: draft
---

# Cryptography

Dew uses the same core primitives as Ethereum so wallets and libraries interoperate without custom crypto stacks.

## Hash function

| Use                                     | Algorithm                                            |
| :-------------------------------------- | :--------------------------------------------------- |
| Addresses, many Ethereum-compat digests | **Keccak-256** (Ethereum variant, not FIPS SHA3-256) |
| Block / tx identifiers                  | Keccak-256 over the canonical encoding               |

## Key pairs

| Field                     | Spec                           |
| :------------------------ | :----------------------------- |
| Algorithm                 | ECDSA on **secp256k1**         |
| Private key               | 32 bytes, CSPRNG               |
| Public key (uncompressed) | 65 bytes: `0x04 \|\| X \|\| Y` |
| Public key (compressed)   | 33 bytes: `0x02/0x03 \|\| X`   |

## Signatures

- **EVM transactions**: standard Ethereum ECDSA with recovery id; chain ID per EIP-155 / EIP-1559.
- **Consensus votes / proposals**: same curve; domain separation MUST differ from user-tx signing (implementation prefixes or distinct signed payloads).
- **DewTx (Phase B)**: compact `[R \|\| S \|\| V]` (65 bytes) over a domain-separated digest of the native payload.

## Address derivation

See [Addresses](./addresses.md). Summary:

$$
\text{Address} = \text{Keccak-256}(\text{uncompressedPubKey}[1:])[12:32]
$$

(last 20 bytes of the hash of the 64-byte XY coordinates).

## Implementation notes (Go)

- Prefer audited libraries (e.g. go-ethereum `crypto` packages) for secp256k1 and Keccak.
- Never roll your own elliptic curve arithmetic.
- Zeroize private keys in memory where practical; store keys encrypted at rest (`dewcli` keystore).
