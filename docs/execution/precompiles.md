---
title: Precompiles
description: Standard Ethereum precompiles and Dew system contracts.
category: execution
order: 40
status: stable
---

# Precompiles

Precompiles are native implementations exposed at fixed addresses, callable like contracts but executed in Go.

## Standard Ethereum set (Cancun)

| Address | Name |
| :--- | :--- |
| `0x01` | ecrecover |
| `0x02` | SHA2-256 |
| `0x03` | RIPEMD-160 |
| `0x04` | identity |
| `0x05` | modexp |
| `0x06` | ecAdd (alt_bn128) |
| `0x07` | ecMul |
| `0x08` | ecPairing |
| `0x09` | blake2f |
| `0x0a` | point evaluation (KZG / EIP-4844 related) |

Gas costs: match Cancun unless a documented exception exists.

## Dew system precompiles

Reserved starting at `0x100`. Enabled when the executor feature flag is on
(`Executor.EnableDewPrecompiles(true)`, default on; matches `params.DefaultEnableDewPrecompiles`).

Formal registry (addresses, status, live-map flags): `vm.DewPrecompileSlots()` · slot policy: [Addresses — Precompile slots](../protocol/addresses.md#precompile-slots-evm-space).

| Address | Name | Gas | Status | Live map |
| :--- | :--- | ---: | :--- | :--- |
| `0x100` | Native transfer | 3_000 fixed | **Active** — forward CALLVALUE to recipient | Yes |
| `0x101` | Native swap / orderbook | method-based | **Flagged** — place/cancel/fill when native-swap on | Yes |
| `0x102` | Staking entrypoint | method-based | **Flagged** — bond/unbond/queries/jail when staking on | Yes |
| `0x103+` | (unallocated) | — | Free — next assignable is `0x103` | — |

**Status meanings:** *Active* = full methods when Dew precompiles on · *Reserved* = address held, empty account (CALL does not run native code) · *Flagged* = in the live map but methods gated (`EnableStaking` / `EnableNativeSwap`).

### `0x100` — Native transfer

Useful bridge from Solidity into native DEW movement without an ERC-20 hop.

**Call convention**

- `to` = `0x0000…0100`
- `value` = amount of native DEW to forward
- `data` = 20-byte recipient address (exactly 20 bytes)

**Semantics**

1. EVM transfers `value` from caller to `0x100` (standard CALL value rules).
2. Precompile moves the full balance of `0x100` to `recipient`.
3. Returns `uint256` amount forwarded (ABI left-padded 32 bytes).

**Rules:** fixed gas; deterministic; reverts on malformed input (not 20 bytes). Feature flag off → address is a normal empty account (value sits at `0x100`, not forwarded).

### System contracts (lab / DX)

For operators and demos, treat the live/flagged Dew slots as **system contracts**:

| Address | Label | Default public-testnet-v1 |
| :--- | :--- | :--- |
| `0x100` | Native transfer | On (with Dew precompiles) |
| `0x101` | Orderbook | Methods **off** unless `--native-swap` |
| `0x102` | Staking | Methods **off** unless `--staking` |

Explorer/RPC may list these as known system addresses without an indexer (optional badge). Full slot policy: [Addresses](../protocol/addresses.md#precompile-slots-evm-space).

### `0x102` — Staking entrypoint (Phase C4)

Enabled when Dew precompiles are on **and** `Executor.EnableStaking(true)` / `Node.SetStakingEnabled(true)`. Default **off** until operators opt in (`params.DefaultEnableStaking = false`).

**Byte layout** (fail-closed; not full Solidity ABI):

| Method | Input | Value | Gas | Effect |
| :--- | :--- | :--- | --: | :--- |
| `0x00` bond | `[0x00]` | self-stake amount | 50_000 | Escrow CALLVALUE; credit **immediate CALL payer** (nested `msg.sender`) |
| `0x01` unbond | `[0x01 \|\| amount uint256]` | 0 | 40_000 | Reduce **tx.origin** stake; queue until unbonding period |
| `0x02` getSelfStake | `[0x02 \|\| addr20]` | 0 | 2_000 | Return stake uint256 |
| `0x03` getVotingPower | `[0x03 \|\| addr20]` | 0 | 2_000 | 0 if jailed / not candidate |
| `0x04` activeCount | `[0x04]` | 0 | 2_000 | Top-K set size |
| `0x05` activeAt | `[0x05 \|\| index uint256]` | 0 | 2_000 | Address at rank |
| `0x06` jail | `[0x06 \|\| voteA(114) \|\| voteB(114)]` | 0 | 30_000 | Dual-vote double-sign verify → jail offender |
| `0x07` isJailed | `[0x07 \|\| addr20]` | 0 | 2_000 | 0/1 |
| `0x08` withdraw | `[0x08]` | 0 | 40_000 | Claim matured unbond to **tx.origin** (D3c) |
| `0x09` pendingUnbond | `[0x09 \|\| addr20]` | 0 | 2_000 | `amount` (32) \|\| `unlockAt` unix (32) |
| `0x0a` delegate | `[0x0a \|\| validator 20]` | amount | 50_000 | Escrow CALLVALUE; credit **value-payer** → validator (no self-delegate) |
| `0x0b` undelegate | `[0x0b \|\| validator 20 \|\| amount u256]` | 0 | 40_000 | Reduce live del of **tx.origin**; queue per (val, del) |
| `0x0c` withdrawDelegation | `[0x0c \|\| validator 20]` | 0 | 40_000 | Claim matured undelegation to **tx.origin** |
| `0x0d` setCommission | `[0x0d \|\| bps u256]` (0–10000) | 0 | 30_000 | Store commission for **tx.origin** (storage only; no reward split v1) |
| `0x0e` getDelegation | `[0x0e \|\| val 20 \|\| del 20]` | 0 | 2_000 | Live amount |
| `0x0f` getCommission | `[0x0f \|\| val 20]` | 0 | 2_000 | bps |
| `0x10` getDelegatedTotal | `[0x10 \|\| val 20]` | 0 | 2_000 | Sum live del |
| `0x11` pendingUndelegation | `[0x11 \|\| val 20 \|\| del 20]` | 0 | 2_000 | `amount\|\|unlockAt` |

**Jail vote wire (114 bytes each):** `type(1) || height(8 BE) || round(8 BE) || blockHash(32) || signature(65)`. Both votes must verify; same type/height/round/validator; distinct hashes.

**Actor rules (S4, fail-closed):**

| Method | Actor | Nested CALL |
| :--- | :--- | :--- |
| `0x00` bond / `0x0a` delegate | Immediate value-payer (`Transfer` hook) | **Correct** — contract can bond/delegate with CALLVALUE |
| `0x01` unbond / `0x08` withdraw / `0x0b` undelegate / `0x0c` withdrawDelegation / `0x0d` setCommission | Always **tx.origin** | **Not** intermediate `msg.sender` — go-ethereum precompile `Run` has no call stack |
| Queries / jail | Explicit address in input (or evidence) | N/A |

Implication: a contract that nested-bonds to itself **cannot** unbond/withdraw that stake via a nested zero-value CALL under this freeze. Prefer top-level EOA calls for unbond/withdraw, or wait for a hardfork (call-stack or explicit address arg — ABI change). Covered by `TestStakingUnbondWithdraw_ActorIsTxOrigin_NestedForwarder`.

**Rules:** min **self-stake** `100_000 * 10^18` wei (**public-testnet-v1** defaults; lab tests may lower via config); active set = top `K` (default 100) by **voting power** (`self + delegated`) among candidates with **SelfStake ≥ min** and not jailed (pure delegation cannot enter). **Unbonding** / undelegation use block timestamp + genesis `unbondingPeriodSeconds` (default 604_800); funds stay at `0x102` until withdraw after unlock. **Commission** is stored (bps) only — reward/tip split **not** implemented (tokenomics draft). **Slash burn percentages** are **not** applied on-chain yet ([tokenomics](../economics/tokenomics.md), [slashing](../consensus/slashing.md)); jail-only today. Design: [0x102 delegation](../superpowers/specs/2026-07-15-0x102-delegation-design.md). See [Public testnet freeze](../ops/public-testnet.md) and [D3 scale — D3c](../scale/d3-scale.md).

Module state: `core/native/staking.go` storage under address `0x102`.

### `0x101` — Native limit orderbook

Enabled when Dew precompiles are on **and** `Executor.EnableNativeSwap(true)` / `Node.SetNativeSwapEnabled(true)` / `dew run --native-swap`. Default **off** (`params.DefaultEnableNativeSwap = false`).

**Hardfork:** registering this slot (reserved empty → flagged precompile) is a consensus-visible change. See [hf-0x101-orderbook.md](../protocol/hf-0x101-orderbook.md). Design: [0x101 orderbook](../superpowers/specs/2026-07-15-0x101-orderbook-design.md).

**Model (v1):** single quote = native DEW; base = any ERC-20 address per order. Limit only; **fill by `orderId`** (no auto best-price walk). Max **64** open orders per maker. Actor = **tx.origin** for place/cancel/fill.

**Byte layout** (fail-closed; not full Solidity ABI):

| Method | Byte | Input | Value | Gas | Effect |
| :--- | :--- | :--- | :--- | --: | :--- |
| Place | `0x00` | `side u8` (0=buy,1=sell) \|\| `baseToken 20` \|\| `priceX18 u256` \|\| `baseAmount u256` | **Buy:** exact `quoteLock` ceil; **Sell:** 0 | 50_000 | Escrow + return `orderId` |
| Cancel | `0x01` | `orderId u256` | 0 | 30_000 | Refund maker (DEW and/or ERC-20) |
| Fill | `0x02` | `orderId u256` \|\| `baseAmount u256` | **Hit sell:** ≥ floor quote; **Hit buy:** 0 | 80_000 | Partial/full fill; return `baseFilled`\|\|`quotePaid` |
| GetOrder | `0x03` | `orderId u256` | 0 | 3_000 | Packed order (9×32) or empty |
| BestBid | `0x04` | `baseToken 20` | 0 | 3_000 | `price`\|\|`orderId` (zeros if none) |
| BestAsk | `0x05` | `baseToken 20` | 0 | 3_000 | same |
| GetEscrow | `0x06` | `maker 20` \|\| `baseToken 20` | 0 | 3_000 | `baseBal`\|\|`quoteBal` |

**Price:** `priceX18` = quote wei per 1e18 base. **Fills:** `quote = floor(base * priceX18 / 1e18)`. **Buy place lock:** `ceil(base * priceX18 / 1e18)`.

**ERC-20:** sell place / buy fill pull base via nested `transferFrom` (maker/taker must `approve` `0x101`). No `CreditBase` in v1. Mock lab token includes `approve`/`transferFrom` (`TokenCreationBytecode`).

**Flag off:** methods revert (`native swap: not enabled`). Module state: `core/native/orderbook.go` under address `0x101`. Gas: `params/orderbook.go`.

### Design rules for custom precompiles

1. Fixed gas schedule (no unbounded native work free of gas)
2. Deterministic output for given input + state
3. Document ABI / byte layout before any testnet uses them
4. Prefer fail-safe reverts over panics in Go
5. Assign next free slot; do not reuse retired addresses; reserved slots stay out of the live map until hardfork activation

## Implementation

`core/vm/precompiles.go` clones Cancun precompiles and registers **live-map** Dew addresses (`0x100`, `0x101`, `0x102`) via `evm.SetPrecompiles` when Dew precompiles are enabled. `0x101` / `0x102` methods remain gated (`EnableNativeSwap` / `EnableStaking`). `DewPrecompileAddresses()` / `DewPrecompileSlots()` are the single registry used for access-list warming and docs alignment. Unallocated slots stay out of the map (empty account semantics).
