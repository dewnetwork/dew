---
title: Public testnet freeze
description: public-testnet-v1 freeze surface, faucet policy, bootnodes, and feature flags.
category: ops
order: 50
status: stable
---

# Public testnet freeze (Phase C6)

Tag: **`public-testnet-v1`** (`params.PublicTestnetFreezeTag`).

C6 is a **release gate**, not a feature dump. Wire formats, fee floors, precompile addresses, and mempool/RPC abuse limits below are **frozen** for the first public testnet. After this freeze, prefer genesis/config changes over wire churn. Mainnet still requires an **external** audit of consensus + VM bridge + crypto (see `agents/debt.md`).

Private multi-host ops remain in [Private multi-host testnet](./private-testnet.md). Local DX: [Devnet](./devnet.md).

## Live network (path B)

**Status:** **public-testnet-v1** is live on controlled single-host RPC (path B), deployed July 2026.

| Surface | URL |
| :------ | :---- |
| JSON-RPC | `https://rpc-dew.fadosoft.com` |
| Block explorer | `https://explorer-dew.fadosoft.com` |
| Faucet | `https://faucet-dew.fadosoft.com` |
| Guestbook SPA | `https://guestbook-dew.fadosoft.com` |
| Guestbook contract | `0x83bB4E539BE46503481E66094b01b854990BF84a` (override SPA via `PUBLIC_GUESTBOOK`) |
| Mock assets (test only) | See table below — explorer `PUBLIC_KNOWN_TOKENS` |
| Bootnodes | **n/a** (path B — no public P2P dial list) |

### Live mock ERC-20 basket (path B)

Mintable test tokens for dApps / MetaMask / explorer balances. **Not** real USDT/USDC/DAI/WETH/WBTC. Redeploy changes only ops env + docs (not wire freeze).

| Symbol | Decimals | Address |
| :--- | ---: | :--- |
| USDT | 6 | `0x43d08b71B0A5626a9D0eB607C2aE5277255cFbC8` |
| USDC | 6 | `0xacDd0BF26C96879b4c4CeE8F92928f1760F07119` |
| DAI | 18 | `0x9813B1738eb2982F3D0C1E22F9C7c4F07B670a1B` |
| WETH | 18 | `0x5b88b2ab38920197328f9D90BaAf29cb91F7E3CB` |
| WBTC | 8 | `0xdB6E09cb954Ae645C815Bc941bDfD85D4144166E` |

```text
PUBLIC_KNOWN_TOKENS=USDT:0x43d08b71B0A5626a9D0eB607C2aE5277255cFbC8,USDC:0xacDd0BF26C96879b4c4CeE8F92928f1760F07119,DAI:0x9813B1738eb2982F3D0C1E22F9C7c4F07B670a1B,WETH:0x5b88b2ab38920197328f9D90BaAf29cb91F7E3CB,WBTC:0xdB6E09cb954Ae645C815Bc941bDfD85D4144166E
```

Deployer (owner / mint): `0x0daEeD75872d3277C012B9f7762549c64B403F76`. Contracts + scripts: [examples/foundry](../../examples/foundry/) · [examples/hardhat](../../examples/hardhat/) · [Recipe 1c](./recipes.md#recipe-1c--mock-assets-basket-testnet).

| Item | Value |
| :--- | :---- |
| Chain ID | `2205` (`0x89d`) |
| Symbol | DEW |
| Faucet mode | **captcha** — 1 DEW / address / 24h · 10 / IP / hour |
| Smoke | `node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com` |
| Browser demo | [Try public testnet (5 minutes)](./try-public.md) |
| Guestbook product | [Guestbook demo](../product/guestbook.md) |

MetaMask: custom network RPC = JSON-RPC URL above; block explorer URL = explorer base only (no path suffix). Path A (multi-host validators + bootnodes) is [D3](../build/phases.md#d3--scale-when-needed-path-a--c4--audit) — see [D3 scale](../scale/d3-scale.md) — not required for the current deployment.

**Demo apps are ops surface**, not wire-freeze constants: redeploying Guestbook or mock assets changes only `PUBLIC_GUESTBOOK` / `PUBLIC_KNOWN_TOKENS` + publish text, not chain ID or fee floors.

## Freeze table

| Surface | Value | Code / source |
| :------ | :---- | :------------ |
| Freeze tag | `public-testnet-v1` | `params.PublicTestnetFreezeTag` |
| Chain ID | `2205` (`0x89d`) | `params.PublicTestnetChainID`, genesis — not `2026` (Edgeless / EwEth on chainlist) |
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
| Staking `0x102` live methods | **off** | Operators opt in; unbonding enforced; BFT set rotation still residual |
| Parallel execution | available | Equivalence-tested; not required for freeze |
| P2P encrypt | **on** | Cleartext only with explicit allow flag (dev) |

## Faucet policy

| Rule | Guidance |
| :--- | :------- |
| Dev Anvil keys | **Never** as public faucet or validator keys |
| Rate limit | Cap per IP / address / day (operator-chosen; document on launch page) |
| Amount | Small enough for deploy + a few transfers; not economic yield |
| Captcha / allowlist | Required for public open mint (`allowlist` or `captcha` mode) |
| Incentives | Optional; if any, separate from faucet and time-bounded |

Faucet is **ops**, not consensus. Disable faucet independently of validators.

**In-repo service (Phase D2):** `cmd/dewfaucet` — see [Production faucet](../product/faucet.md). Operator defaults: **1 DEW** / drip, **1 / address / 24h**, **10 / IP / hour**; modes `allowlist` (default) · `captcha` · `dev` (private only).

## Bootnodes

**Current deployment (path B):** bootnodes are **n/a** — operators expose JSON-RPC via TLS edge only; there is no public P2P dial list.

**Path A (future):** publish a short list of stable dial addresses for public peers:

```
# Example shape (replace when Path A launches)
enode-style or dew multiaddr — host:port + node identity
bootnode-0.example:30303
bootnode-1.example:30304
```

Requirements:

1. Same genesis hash / chain ID on every honest node  
2. Encrypted P2P default  
3. At least one public non-validator RPC (or documented RPC providers)  
4. Rotate bootnodes without changing chain ID  

Path A hostnames are operator-owned; this runbook freezes the **process**, not a specific multi-host topology.

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

Full one-page checklist (ports, env, compose, publish template): [Launch checklist](./launch-checklist.md). Packaging samples: [deploy/node/](../../deploy/node/) and [deploy/faucet/](../../deploy/faucet/).

**Fast path (single host, controlled RPC):** [deploy/node/public-rpc-single-host.md](../../deploy/node/public-rpc-single-host.md).

1. Generate dedicated keys (validators, bootnodes, faucet) — no Anvil reuse  
2. Distribute frozen genesis (chain ID 2205, alloc, initialValidators)  
3. Start ≥ 3 validators + optional RPC with encrypted P2P  
4. Publish RPC URL, chain ID, bootnodes, faucet rules, explorer base or `(none)` ([Block explorer](../product/block-explorer.md))  
5. Confirm feature flags match the table above  
6. Run chaos smoke on private staging first (`go test ./devnet/ -run Chaos`)  
7. Monitor RPC errors / mempool rejects; be ready to disable native/staking flags  

### Emergency stop

1. Stop public RPC (and faucet)  
2. Disable native path / staking if module bug  
3. Keep BFT validators online if only RPC is abused  
4. Re-genesis only if wire/root freeze is intentionally broken (coordinate publicly)  

## Residual (not C6 blockers)

Tracked in `agents/debt.md`; D3 implementation spec: [D3 scale](../scale/d3-scale.md). Items: fee auction, nonce-gap queue, D3c staking residuals, D3d Path A bootnodes, external mainnet audit. (D3a multiproc long-run residual closed July 2026.)

## Related

- [Launch checklist](./launch-checklist.md)  
- [Block explorer (web)](../product/block-explorer.md) — public Explorer URL, deep links  
- [Production faucet](../product/faucet.md) — D2 `dewfaucet`  
- [Phases](../build/phases.md) — C6 acceptance  
- [D3 scale](../scale/d3-scale.md) — post-freeze technical workstreams
- [Genesis](../economics/genesis.md)  
- [Gas and fees](../execution/gas-and-fees.md)  
- [Security principles](../security/security-principles.md) — public testnet bar  
- [Private testnet](./private-testnet.md)  
- [deploy packaging](../../deploy/README.md)  
