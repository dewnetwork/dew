# Hardfork: `0x101` native limit orderbook (registration)

**Status:** Spec for binary upgrade / operator coordination · **Not activated** on public path B until operators ship a build that registers the slot and (optionally) set `--native-swap`.

**Design:** [0x101 orderbook design](../superpowers/specs/2026-07-15-0x101-orderbook-design.md) · **Plan:** [implementation plan](../superpowers/plans/2026-07-15-0x101-orderbook.md)

## Consensus-visible change

| Before (public-testnet-v1 freeze) | After this hardfork binary |
| :--- | :--- |
| `0x101` **Reserved** — empty account; CALL succeeds; value sits at address | `0x101` **Flagged** — precompile in live map; methods run when `EnableNativeSwap`; when flag **off**, method calls **revert** (`native swap: not enabled`) |

Upgrading nodes without a coordinated cutover changes CALL success/failure at `0x101`. Treat as a **hardfork** under freeze rules ([addresses.md](addresses.md#allocation-rules)).

## Feature flag

- `EnableNativeSwap` / `dew run --native-swap` — default **false**.
- Lab may enable on private nets. Public path B remains flag-off until a later policy decision.

## Method / gas / rounding

See [precompiles.md](../execution/precompiles.md) and the design doc.

| Op | Gas |
| :--- | ---: |
| Place | 50_000 |
| Cancel | 30_000 |
| Fill | 80_000 |
| Views (GetOrder / BestBid / BestAsk / GetEscrow) | 3_000 |

Constants: `params/orderbook.go`.

- Quote math: floor `(base * priceX18) / 1e18` on fills; buy place locks **ceil** so full size is coverable.
- Actor: **tx.origin** for place/cancel/fill (nested CALL limits like staking zero-value methods).
- Base ERC-20: `transferFrom` / `transfer` via nested EVM call (no `CreditBase` in v1).
- Max open orders per maker: **64**.
- Fill is by **orderId** only (no auto best-price walk in v1).

## Activation checklist

1. [x] Implementation landed on branch; unit + precompile tests green.
2. [x] Docs: addresses + precompiles status **flagged**.
3. [x] Lab path covered by `go test ./core/native/ ./core/vm/ -run Orderbook` (place/cancel/fill/views + flag-off); private net `--native-swap` optional operator demo.
4. [ ] Public: operator binary upgrade coordination; keep `--native-swap` off unless intentionally enabling.
