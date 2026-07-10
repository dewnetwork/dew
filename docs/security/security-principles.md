---
title: Security Principles
description: Engineering practices that make Dewchain safer than a rushed chain fork.
category: security
order: 20
status: draft
---

# Security Principles

## 1. Prefer boring crypto

Use standard secp256k1 + Keccak and audited libraries. No novel curves in Phase A.

## 2. Domain separation

User transactions, consensus votes, and DewTx MUST hash different domains so a signature cannot be replayed across message types.

## 3. Fail closed

Invalid signature, wrong chain ID, bad state root, incomplete DewTx access list → reject. Never “best effort” apply partial state from a failed tx.

## 4. Determinism

Given parent state + block body, every honest full node computes the same roots. Parallel execution is unsafe until proven equivalent to sequential.

## 5. Least privilege in node design

- RPC should not require validator keys on the same host in production (encourage remote signer later)
- Separate filesystem permissions for key material

## 6. Resource limits

Cap:

- P2P message size
- `eth_call` gas
- Mempool per-sender count
- Concurrent RPC connections

Cheaper gas must not mean free infinite CPU.

## 7. Accountability over anonymity for validators

Bonded identities + evidence of equivocation beat pseudonymous longest-chain griefing for this design.

## 8. Secure the path to production

| Stage           | Bar                                                  |
| :-------------- | :--------------------------------------------------- |
| Local dev       | Happy path tests                                     |
| Private testnet | Multi-validator, chaos restart                       |
| Public testnet  | External fuzzing, RPC abuse tests                    |
| Mainnet         | Spec freeze, audit of consensus + VM bridge + crypto |

## 9. Cheaper ≠ weaker fee security

Lower gas prices still require:

- Nonce enforcement
- Base fee floor anti-spam if needed
- Minimum priority fee config for public mempools

## 10. Document secrets and ops

Runbooks for key rotation, jail/unjail, and emergency stop (feature disable flags) before public incentives go live.
