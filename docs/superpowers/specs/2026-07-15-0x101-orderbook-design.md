# `0x101` Native limit orderbook — design

**Status:** Approved design (2026-07-15) · **Not implemented** · **Not live** under `public-testnet-v1`  
**Track:** Protocol (precompile) + future hardfork  
**Slot:** `0x101` · name `native_swap` (registry) · today **Reserved** / `LiveInMap: false`  
**Parents:** [addresses — precompile slots](../../protocol/addresses.md#precompile-slots-evm-space) · [precompiles.md § 0x101](../../execution/precompiles.md) · [agents/debt.md](../../../agents/debt.md)

## Problem

Slot `0x101` is reserved for “native swap / orderbook” but has no ABI, gas table, storage layout, or activation plan. Shipping a live implementation without a design would break the freeze allocation rules and risk consensus bugs (matching non-determinism, custody leaks).

## Goals

1. Lock **Approach A**: on-chain **limit order book** with escrow, place / cancel / fill, price-time priority.  
2. Define **byte layouts**, gas **budget ranges**, and **state keys** so an implementation PR has no ABI invention.  
3. Define **activation** path: hardfork doc + feature flag (staking pattern), default **off** even after registration.  
4. Keep behavior **deterministic** and serial-equivalent under Dew-PE.

## Non-goals (this design / first implementation)

- AMM / constant-product (`x*y=k`) as the primary model.  
- Off-chain matching engine or order gossip.  
- Multi-asset markets beyond **one base ERC-20 + native DEW quote** in v1.  
- Partial-fill priority fees, stop orders, iceberg, or batch auctions.  
- Activating live map on public-testnet-v1 **without** a hardfork document.  
- Replacing Uniswap-style ERC-20 pools (those remain user contracts).

## Approach (locked): A — Limit book + escrow

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| Market v1 | Single pair: **base** = one ERC-20 address (config or method param) × **quote** = native DEW (wei) | Matches native precompile story; avoids multi-market registry |
| Custody | Escrow balances **at `0x101`** (like stake at `0x102`) | Clear custody; cancel returns to maker |
| Order type | Limit only; buy (quote→base) or sell (base→quote) | Classic CLOB |
| Matching | In-transaction against resting book; price-time FIFO | Deterministic; no external sorter |
| Price | `uint256 priceX18` (quote wei per 1e18 base) | Fixed-point; no floats |
| Size | Base amount in token **raw units** (respect decimals off-chain) | No on-chain decimals oracle in v1 |
| Actor | **tx.origin** for place/cancel/fill auth where call stack missing (document like staking edges) | Same precompile constraint as `0x102` |
| Gate | `EnableNativeSwap` (default false) + hardfork activation | Lab can test without public risk |

### Pair scope (v1 refine)

**Option chosen:** **Open base token** — each order carries `baseToken` (20 bytes). Book is partitioned by `baseToken` (separate order ids per market). Quote always native DEW.

Rationale: demos can use Path B mock USDT/USDC without a genesis config change. Storage cost: map per token.

**v1 limits (abuse):** max open orders per maker **64**; max fills per tx **16**; max book walk gas via fixed steps.

## Method byte layout

Input style matches staking: first byte method, then fixed BE fields. No Solidity ABI encoder required for core methods (wrappers can pack).

| Method | Byte | Input | Payable | Output |
| :--- | :--- | :--- | :--- | :--- |
| `Place` | `0x00` | `side u8` (0=buy,1=sell) \|\| `baseToken 20` \|\| `priceX18 u256` \|\| `baseAmount u256` | **Buy:** CALLVALUE = quote escrow (exact required); **Sell:** CALLVALUE=0, base pulled via allowance/transfer-in pattern — see Custody | `orderId u256` |
| `Cancel` | `0x01` | `orderId u256` | no | empty |
| `Fill` | `0x02` | `orderId u256` \|\| `baseAmount u256` (max fill) | **Taker buy** (hits sell): CALLVALUE quote; **Taker sell** (hits buy): base transfer-in | `baseFilled u256` \|\| `quotePaid u256` |
| `GetOrder` | `0x03` | `orderId u256` | no | packed order or empty |
| `BestBid` | `0x04` | `baseToken 20` | no | `priceX18` \|\| `orderId` (or zeros) |
| `BestAsk` | `0x05` | `baseToken 20` | no | same |
| `GetEscrow` | `0x06` | `maker 20` \|\| `baseToken 20` | no | `baseBal u256` \|\| `quoteBal u256` |

Malformed input → revert (not panic). Unknown method byte → revert `"method"`.

### Side semantics

| Side | Maker locks | Fill (taker) |
| :--- | :--- | :--- |
| **Buy** (0) | Quote DEW at `0x101` | Taker provides **base** ERC-20; receives quote |
| **Sell** (1) | Base ERC-20 at `0x101` | Taker provides **quote** DEW (CALLVALUE); receives base |

### ERC-20 pull (sell place / buy fill)

Precompiles cannot easily do `transferFrom` without standard ERC-20 calls from Go executor. **Implementation requirement:** use existing EVM CALL into token from precompile host (as other chains do) **or** require prior `transfer` to `0x101` + credit mapping (two-step).  

**Locked for v1 implementation:** **two-step credit** (simpler, fewer reentrancy surprises):

1. User `ERC20.transfer(0x101, amount)` (or DEW CALLVALUE for quote).  
2. `Place`/`Fill` consumes **pending credit** for `tx.origin` tracked in native module state, or uses CALLVALUE for DEW.  

Document credit method:

| Method | Byte | Input | Notes |
| :--- | :--- | :--- | :--- |
| `CreditBase` | `0x07` | `baseToken 20` | After token sits at `0x101`, credit `tx.origin` pending base (balance-of-precompile accounting must be careful — prefer explicit deposit via transfer + `CreditBase` that snapshots increase). |

**Safer alternative for implementers:** implement `Place` sell only after internal `transferFrom` helper in executor if codebase already supports token calls; if not, stick to two-step. Spec allows either; first PR must pick one and tests lock it.

**Recommendation for first PR:** DEW quote via CALLVALUE only; base via **transferFrom** if `core/vm` can issue ERC-20 calls; otherwise two-step + tests.

## Order record

```text
Order {
  id          uint64   // monotonic
  maker       address
  baseToken   address
  side        uint8
  priceX18    uint256
  baseOpen    uint256  // remaining
  baseOrig    uint256
  createdAt   uint64   // block timestamp or number (prefer number for determinism)
  status      uint8    // open=0, filled=1, cancelled=2
}
```

Matching rule for taker `Fill`:

- Order must be `open`.  
- Taker side is opposite of maker.  
- Price: buy maker accepts fill if taker sell price ≤ maker price? **Limit buy** rests at max price willing to pay; **limit sell** at min.  
  - Fill against **sell** order: taker pays `baseFilled * priceX18 / 1e18` quote (ceiling policy documented).  
  - Fill against **buy** order: taker receives that quote formula.  
- Partial fills reduce `baseOpen`; zero → status filled.  
- **Price-time:** when introducing `MatchBest` later; v1 explicit `orderId` fill is enough (no auto-walk).  

**v1 simplification (locked):** only **Fill by orderId** (no automatic best-price walk). `BestBid`/`BestAsk` are views for UI only. Reduces PE conflict surface.

## Storage (native module sketch)

Package: `core/native/orderbook.go` (new), state under account `0x101` or dedicated prefix in flat state / SMT.

| Key idea | Value |
| :--- | :--- |
| `meta/nextOrderId` | u64 |
| `order/{id}` | RLP or fixed pack of Order |
| `open/{baseToken}/{side}/{priceX18}/{id}` | marker for book index (optional v1 — can scan if demos tiny) |
| `escrowQuote/{maker}` | u256 |
| `escrowBase/{maker}/{token}` | u256 |

Book index for BestBid/Ask: maintain best price pointers per token/side updated on place/cancel/fill (O(1) views).

## Gas (TBD numbers at activation; design budgets)

| Op | Budget class |
| :--- | :--- |
| Place | O(1) ~ 25k–80k native gas + callvalue handling |
| Cancel | O(1) ~ 20k–50k |
| Fill | O(1) ~ 40k–100k + ERC-20 transfer gas if any |
| Views | ~ 1k–5k |

Final numbers freeze in hardfork doc + `params/`. Must be fixed `RequiredGas` or schedule by method byte (like staking).

## Activation plan (not part of this design’s code ship)

1. **This design doc** (done when approved/committed).  
2. **Hardfork doc** under `docs/protocol/` or `docs/ops/`: chain ID rule, activation height/timestamp, Enable flag default.  
3. Implementation PR: `LiveInMap: true`, status `flagged`, `EnableNativeSwap` default **false**.  
4. Lab: private net tests + Foundry-style call scripts.  
5. Public: only after hardfork activation + flag policy.

**public-testnet-v1 today:** remains reserved / empty account. No code registration without hardfork.

## PE / determinism

- No wall-clock; use block number for `createdAt`.  
- No map iteration order dependence in consensus results (fills are explicit orderId).  
- Escrow math integer only; document rounding (**floor quote to maker favor or protocol favor — lock: floor to benefit maker on sell fills, document in hardfork**).

## Testing plan (future implementation)

1. Unit: place buy with value → escrow; cancel refunds.  
2. Place sell + fill with value → token and DEW move correctly.  
3. Partial fill; double cancel reverts; fill cancelled reverts.  
4. PE: conflicting fills serial-equivalent.  
5. Flag off: methods revert or address not in map.

## Docs to update on implementation

- `docs/execution/precompiles.md` — status flagged/active  
- `docs/protocol/addresses.md` — status row  
- `params/freeze.go` — if needed  
- `agents/debt.md` — move from “needs design” to “implemented, gated”  
- Hardfork doc (required for public activation)

## Acceptance (design phase)

- [x] Approach A locked (limit + escrow + fill-by-id)  
- [x] Method table + order fields  
- [x] Activation / freeze rules  
- [x] Explicit non-goals  
- [ ] Implementation + hardfork (separate plans)

## Open points for implementation plan (not blockers for this design)

1. ERC-20 `transferFrom` vs two-step credit (executor capability).  
2. Exact gas constants.  
3. Rounding mode unit tests.  
4. Whether `baseToken` allowlist (params) is needed for public nets.

## Related

- [Precompiles](../../execution/precompiles.md)  
- [Addresses — slots](../../protocol/addresses.md#precompile-slots-evm-space)  
- Staking precompile pattern: `core/vm/precompiles.go`, `core/native/staking.go`  
- Debt residual: live `0x101` orderbook  
