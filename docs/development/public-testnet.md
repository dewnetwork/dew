---
title: Public testnet freeze
description: public-testnet-v1 freeze surface, faucet policy, bootnodes, and feature flags.
category: development
order: 50
status: draft
---

# Public testnet freeze (Phase C6)

Tag: **`public-testnet-v1`** (`params.PublicTestnetFreezeTag`).

C6 is a **release gate**, not a feature dump. Wire formats, fee floors, precompile addresses, and mempool/RPC abuse limits below are **freeze candidates** for the first public testnet. After this freeze, prefer genesis/config changes over wire churn. Mainnet still requires an **external** audit of consensus + VM bridge + crypto (see `agents/debt.md`).

Private multi-host ops remain in [Private multi-host testnet](./private-testnet.md). Local DX: [Devnet](./devnet.md).

## Freeze table

| Surface | Value | Code / source |
| :------ | :---- | :------------ |
| Freeze tag | `public-testnet-v1` | `params.PublicTestnetFreezeTag` |
| Chain ID | `2026` (`0x7ea`) | `params.PublicTestnetChainID`, genesis |
| Genesis base fee | 1 gwei | `baseFeePerGas` / `ReferenceBaseFeeWei` |
| Block gas limit | 120,000,000 | `DefaultBlockGasLimit` |
| State root | SMT commit-time root | `core/state` (C3) |
| DewTx wire | `0xdf \|\| RLP(signed)` | `types.DewTxType` |
| DewTx domain | `DewTx:v1` | `params.DewTxDomainTag` |
| DewTx flat fee / floor | `2.1e12` wei | `DefaultDewTxFeeWei` / `MinDewTxFeeWei` |
| Precompile `0x100` | native transfer, 3_000 gas | `params` / `core/vm` |
| Precompile `0x102` | staking entrypoint (flagged) | method gas in `params/staking.go` |
| Min gas price (mempool) | 1 gwei | `mempool.DefaultConfig` |
| Min tip (EIP-1559) | 1 wei | same |
| Max tx wire size | 128 KiB | same |
| Mempool global / per-sender | 4096 / 16 | same |
| RPC max body | 1 MiB | `rpc.MaxRequestBodyBytes` |
| RPC max batch | 100 items | `rpc.MaxBatchItems` |
| P2P | encrypted default (C2) | `p2p.Config.Encrypt` |
| Min validator self-stake | 100_000 DEW | `MinValidatorStakeWei` |
| Epoch length | 86_400 blocks | `DefaultEpochLengthBlocks` |
| Active set cap \(K\) | 100 (module default) | `DefaultActiveValidatorCap` |
| Unbonding period | 604_800 s (7d) | `DefaultUnbondingPeriodSeconds` |

Sample `genesis.json` may use a **smaller** `activeValidatorCap` for local 3-validator nets; module default \(K=100\) is the freeze candidate for public operators.

Numbers formerly marked `_tentative_` that appear in this table are **frozen for public-testnet-v1** unless a documented hardfork / re-genesis says otherwise. Tokenomics issuance rates may still be draft (no mainnet claim).

## Feature flags (public-testnet-v1)

| Feature | Default | Notes |
| :------ | :------ | :---- |
| Native DewTx / `dew_*` | **on** | Emergency: `Node.SetNativeEnabled(false)` |
| Dew precompiles `0x100+` | **on** | Emergency: disable precompiles |
| Staking `0x102` live methods | **off** | Operators opt in; unbonding / BFT set rotation still residual |
| Parallel execution | available | Equivalence-tested; not required for freeze |
| P2P encrypt | **on** | Cleartext only with explicit allow flag (dev) |

## Faucet policy

| Rule | Guidance |
| :--- | :------- |
| Dev Anvil keys | **Never** as public faucet or validator keys |
| Rate limit | Cap per IP / address / day (operator-chosen; document on launch page) |
| Amount | Small enough for deploy + a few transfers; not economic yield |
| Captcha / allowlist | Recommended before open faucet |
| Incentives | Optional; if any, separate from faucet and time-bounded |

Faucet is **ops**, not consensus. Disable faucet independently of validators.

## Bootnodes

Publish a short list of stable dial addresses for public peers:

```
# Example shape (replace at launch)
enode-style or dew multiaddr — host:port + node identity
bootnode-0.example:30303
bootnode-1.example:30304
```

Requirements:

1. Same genesis hash / chain ID on every honest node  
2. Encrypted P2P default  
3. At least one public non-validator RPC (or documented RPC providers)  
4. Rotate bootnodes without changing chain ID  

Exact hostnames are **not** frozen in-repo until launch; this runbook freezes the **process**.

## Abuse bar (C6)

| Vector | Mitigation |
| :----- | :--------- |
| Oversized HTTP body | Reject > 1 MiB |
| Oversized JSON-RPC batch | Reject > 100 items |
| Invalid hex / garbage txs | Fail closed (`eth_*` / `dew_*`) |
| Underpriced spam | Mempool fee floors (C1) |
| Oversized tx wire | `MaxTxBytes` |
| Per-sender flood (pending) | `MaxPerSender` |

Tests:

```bash
go test ./tests/security/ -run C6 -count=1
go test ./params/ -run Freeze -count=1
# Optional local fuzz (short):
go test ./core/types/ -fuzz=FuzzDewTxUnmarshal -fuzztime=10s
go test ./rpc/ -fuzz=FuzzDecodeBytes -fuzztime=5s
```

## Operator launch checklist

Full one-page checklist (ports, env, compose, publish template): [Launch checklist](./launch-checklist.md). Packaging samples: [deploy/](../../deploy/).

**Fast path (single host, controlled RPC):** [deploy/public-rpc-single-host.md](../../deploy/public-rpc-single-host.md).

1. Generate dedicated keys (validators, bootnodes, faucet) — no Anvil reuse  
2. Distribute frozen genesis (chain ID 2026, alloc, initialValidators)  
3. Start ≥ 3 validators + optional RPC with encrypted P2P  
4. Publish RPC URL, chain ID, bootnodes, faucet rules  
5. Confirm feature flags match the table above  
6. Run chaos smoke on private staging first (`go test ./devnet/ -run Chaos`)  
7. Monitor RPC errors / mempool rejects; be ready to disable native/staking flags  

### Emergency stop

1. Stop public RPC (and faucet)  
2. Disable native path / staking if module bug  
3. Keep BFT validators online if only RPC is abused  
4. Re-genesis only if wire/root freeze is intentionally broken (coordinate publicly)  

## Residual (not C6 blockers)

Tracked in `agents/debt.md`: fee auction, nonce-gap queue, full unbonding, double-sign verify, ActiveSet → live BFT rotation, multi-host packaging, external mainnet audit.

## Related

- [Launch checklist](./launch-checklist.md)  
- [Phases](./phases.md) — C6 acceptance  
- [Genesis](../economics/genesis.md)  
- [Gas and fees](../execution/gas-and-fees.md)  
- [Security principles](../security/security-principles.md) — public testnet bar  
- [Private testnet](./private-testnet.md)  
- [deploy packaging](../../deploy/README.md)  
