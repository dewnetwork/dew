# Wave 3 — HTTP Filter API Design

**Status:** Approved design (2026-07-15)  
**Track:** 4 (core node) + API  
**Freeze:** `public-testnet-v1` / chain ID **2205** — RPC-only; no consensus, wire, or precompile changes  
**Parent plan:** [Gap closure Wave 3](../plans/2026-07-14-gap-closure.md#wave-3--filter-api-stretch-optional)

## Problem

After Wave 2 (`eth_subscribe` over WebSocket), modern UIs can push-subscribe. Older HTTP tooling (Hardhat listeners, ethers v5 poll filters, some scripts) still uses:

- `eth_newFilter` / `eth_newBlockFilter` / `eth_newPendingTransactionFilter`
- `eth_getFilterChanges` / `eth_getFilterLogs` / `eth_uninstallFilter`

Docs currently mark these as optional stretch. This design ships a **correct poll semantics** subset so those clients work against Dew without requiring WebSocket.

## Goals

1. Implement Ethereum-compatible filter methods over the existing JSON-RPC HTTP server.
2. Reuse log matching and `node.FilterLogs` / log-index path from Wave 1.
3. Abuse limits consistent with C6 public-testnet posture (TTL, max filters).
4. Keep implementation simple: **cursor-by-height**, no per-filter goroutines.

## Non-goals

- Real `pendingTransactions` stream (mempool push) — stub or empty-only.
- Filter API as a replacement for WebSocket (WS remains preferred for realtime).
- Address secondary index keys (`A|…`) beyond existing block-range log index.
- Changing `eth_getLogs` request shape or limits.
- Wire/consensus/precompile changes.

## Approach (locked)

**Server-owned in-memory filter store** on the RPC `API` (or a small `filterStore` held by `API`):

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| Storage | Process-local map `id → filter` | Single-node Path B; no durable filter state |
| Cursor model | **Last-seen block height** (`lastPolled`) | Deterministic; no goroutine leak; easy tests |
| Log source | `node.FilterLogs(from, to, addrs, topics)` | Reuses O(range) log index + in-memory `allLogs` |
| Block hashes | Load header hash for heights `(lastPolled+1)…tip` | Matches geth-style block filter changes |
| Pending tx filter | Install succeeds; `getFilterChanges` always `[]` | Avoids mempool subscription scope |
| Filter ID | Random 8-byte hex `0x…` (same style as WS sub IDs) | Collision-resistant enough for in-memory map |
| Max filters | **128** process-wide | Abuse control; independent of WS limits |
| Idle TTL | **5 minutes** since last successful access (create / changes / logs / uninstall touch) | Lazy expiry on access; optional sweep on mutate |
| Unknown / expired ID | `getFilterChanges` / `getFilterLogs` → JSON-RPC error; `uninstallFilter` → `false` | Common client expectation |

## Methods

### `eth_newFilter(filterObject)`

Creates a **log** filter.

**Params:** one object (same fields as `eth_getLogs` filter object):

- `address` — optional string or array of addresses  
- `topics` — optional topic levels (OR within level, AND across levels; `null` = wildcard)  
- `fromBlock` / `toBlock` — optional at create time for `eth_getFilterLogs` historical range; **poll path ignores fixed toBlock** and only returns logs for blocks after create cursor (standard poll behavior)

**Cursor at create:** `lastPolled = tip` (current `BlockNumber()`). Historical logs before tip are **not** returned by `getFilterChanges`; clients use `getFilterLogs` or `eth_getLogs` for history.

**Returns:** filter id string `0x…`

### `eth_newBlockFilter()`

No params (or empty array). Cursor `lastPolled = tip`.

**Returns:** filter id.

### `eth_newPendingTransactionFilter()`

Installs a filter of kind `pending`. No real pending stream in v1.

**Returns:** filter id.  
`eth_getFilterChanges` always returns `[]` until a future mempool-notification design.

### `eth_getFilterChanges(filterId)`

| Kind | Result |
| :--- | :--- |
| `logs` | Array of log objects (same shape as `eth_getLogs` items) for blocks `(lastPolled+1)…tip` matching address/topics |
| `blocks` | Array of block hash hex strings for sealed/imported heights in that range |
| `pending` | Always `[]` |

After success: set `lastPolled = tip`, refresh idle timestamp.  
If `tip == lastPolled`: return `[]`.  
If filter missing/expired: error `"filter not found"`.

### `eth_getFilterLogs(filterId)`

Only valid for **log** filters. Returns full match for the filter’s address/topics over:

- Prefer filter’s stored `fromBlock`/`toBlock` if set at create (resolved against current tip tags),  
- Else default `0…latest` via `FilterLogs`.

Does **not** advance the poll cursor (matches common Ethereum client behavior).

For block/pending kinds: error `"not a log filter"`.

### `eth_uninstallFilter(filterId)`

Deletes filter. Returns `true` if removed, `false` if unknown/expired.

## Data model

```text
filterStore:
  mu sync.Mutex
  byID map[string]*installedFilter
  // max 128 after purge of expired

installedFilter:
  id          string
  kind        logs | blocks | pending
  addrs       []Address          // log only
  topics      [][]Hash           // log only
  fromBlock   *tag optional      // log only; for getFilterLogs
  toBlock     *tag optional
  lastPolled  uint64             // inclusive last height returned
  lastAccess  time.Time
```

Attach store to `*API` so HTTP handlers share one store per node process.  
`EnableSubscriptions` already has `api *API`; no second global required.

## Matching and formatting

- Parse address/topics using existing helpers where possible (`eth_getLogs` path or `parseLogsFilter` from `ws.go` — prefer extracting shared parse if duplication is painful; **not required** if copy is small and tested).
- Format log rows identically to `eth_getLogs` (`address`, `topics`, `data`, `blockNumber`, `transactionHash`, `transactionIndex`, `blockHash`, `logIndex`, `removed: false`).
- Block filter: for each height `h` in `(lastPolled+1)…tip`, resolve canonical block hash (existing node APIs: block by number → hash).

## Limits and errors

| Limit | Value |
| :--- | :--- |
| Max concurrent filters | 128 |
| Idle TTL | 5 minutes |
| Body/batch | Existing C6 `MaxRequestBodyBytes` / `MaxBatchItems` |

On create when at capacity after purging expired: error `"too many filters"`.

## Files

| Action | Path |
| :--- | :--- |
| Create | `rpc/filter.go` — store + method handlers |
| Create | `rpc/filter_test.go` — unit/integration against in-process node |
| Modify | `rpc/api.go` — register six methods in `Handlers()` |
| Modify | `docs/api/json-rpc.md` — mark implemented + limits |
| Modify | `docs/superpowers/plans/2026-07-14-gap-closure.md` — Wave 3 checkbox progress |
| Modify | `agents/debt.md` — close optional Filter API residual when done |

No change to `node/` storage schema. No nginx change required (HTTP only).

## Testing

1. **Log filter poll:** start node → `eth_newFilter` → send tx that emits logs / seal → `eth_getFilterChanges` returns logs once → second call empty.  
2. **Block filter:** `eth_newBlockFilter` → seal → changes contains new block hash once.  
3. **Uninstall:** uninstall true; subsequent changes error; second uninstall false.  
4. **getFilterLogs:** log filter returns historical matches without advancing cursor (changes still empty if no new blocks).  
5. **Pending:** new pending filter → changes `[]`.  
6. **Capacity / TTL:** optional unit tests on store with injected clock or short TTL in test helper.

Commands:

```bash
go test ./rpc/ -count=1
go test ./rpc/ ./node/ -count=1
```

## Acceptance

- [ ] All six methods registered and documented  
- [ ] Log + block poll semantics correct under auto-mine / seal  
- [ ] Pending stub documented as empty  
- [ ] Limits documented (128 / 5m)  
- [ ] No wire/consensus change  
- [ ] Debt + gap-plan Wave 3 updated  

## Risks

| Risk | Mitigation |
| :--- | :--- |
| Poll gap if tip jumps many blocks | `FilterLogs` range is fine; cap is existing getLogs practical limits |
| Clock skew / long-idle clients | TTL 5m; clients re-create filter |
| Duplicated parse code | Extract only if both paths diverge in tests |

## Related

- [JSON-RPC](../../api/json-rpc.md)  
- [Gap closure plan](../plans/2026-07-14-gap-closure.md)  
- [Product upgrades Track 4](../../product/upgrades.md#track-4--core-node-upgrades)  
- [agents/debt.md](../../../agents/debt.md)  
