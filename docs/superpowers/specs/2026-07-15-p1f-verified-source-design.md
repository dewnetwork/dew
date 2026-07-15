# P1f — Verified source / ABI (design)

**Status:** Approved design (2026-07-15)  
**Track:** 1 (product surface)  
**Freeze:** `public-testnet-v1` / chain ID **2205** — product only; no consensus, wire, or precompile changes  
**Parent:** [upgrades — P1f](../../product/upgrades.md) · [block-explorer](../../product/block-explorer.md) · [indexer](../../product/indexer.md)

## Problem

Explorer address pages show raw `eth_getCode` bytecode on the Contract tab. There is no way to attach human-readable **ABI** or **source** to a contract address. Tx method decode is limited to a hard-coded selector table (`explorer/src/lib/abi.ts`). After P1e (`dewindex`), durable off-chain metadata can live next to history without growing the node.

## Goals

1. Persist contract **ABI** (+ optional source / name / compiler string) keyed by address.
2. Serve read API for the explorer Contract tab and optional method decode.
3. Allow lab-friendly **register** (POST) with abuse limits; optional shared token.
4. Badge language that does **not** claim cryptographic bytecode match (v1 = “ABI registered”).
5. Work when `PUBLIC_INDEXER_URL` is set; degrade gracefully when unset (no form, no badge).

## Non-goals (v1)

- solc / Sourcify compile-and-match runtime bytecode  
- Multi-file projects, constructor args, libraries, immutable refs  
- IPFS pinning or content-addressed source  
- Proxy / implementation verification  
- Storing source on-chain or in node chaindata  
- Wire freeze changes  

## Approach (locked)

**Extend `dewindex` SQLite + HTTP** (Approach A). Explorer is a client only.

| Decision | Choice | Rationale |
| :--- | :--- | :--- |
| Storage | SQLite table `contracts` in same DB as history | One sidecar ops surface; Path B compose already has indexer |
| Identity | Lowercase `0x` + 40 hex address PK | Match indexer address normalization |
| Semantics | **Registered** (user-submitted ABI/source), not “Verified (match)” | Avoid false Etherscan-class claims without solc |
| Auth | Open POST by default; if `INDEXER_VERIFY_TOKEN` set, require `Authorization: Bearer <token>` or `X-Dew-Verify-Token` | Lab-friendly + optional lock-down |
| Rate limit | Per-IP sliding window: **10 POSTs / 10 minutes**; max body **512 KiB** | Abuse control on public Path B |
| Upsert | Same address POST **replaces** row (last-write-wins) | Simple; no version history in v1 |
| Delete | Out of scope v1 (operator can wipe SQLite row manually) | YAGNI |

## Data model

```sql
CREATE TABLE IF NOT EXISTS contracts (
  address    TEXT PRIMARY KEY,   -- lowercase 0x…
  name       TEXT,               -- optional display name
  abi_json   TEXT NOT NULL,      -- JSON array (ABI)
  source     TEXT,               -- optional single-file Solidity/text
  compiler   TEXT,               -- free-form e.g. "solc 0.8.24"
  created_at INTEGER NOT NULL,  -- unix seconds first insert
  updated_at INTEGER NOT NULL   -- unix seconds last upsert
);
```

Validation on write:

- `address` path must be valid 20-byte hex.  
- `abi` must be a JSON **array** (Ethereum ABI JSON). Reject non-array objects. Max serialized size counts toward body limit.  
- `source` optional string, max length remaining under body budget.  
- `name` optional, max **128** runes.  
- `compiler` optional, max **64** runes.  

## HTTP API

Base: existing indexer listen (e.g. `:8550`). CORS: allow `GET, POST, OPTIONS` and header `Content-Type`, `Authorization`, `X-Dew-Verify-Token`.

### `GET /v1/contract/{addr}`

**200:**

```json
{
  "address": "0x…",
  "name": "Token",
  "abi": [ … ],
  "source": "// SPDX…",
  "compiler": "solc 0.8.24",
  "status": "registered",
  "createdAt": 1720000000,
  "updatedAt": 1720000000
}
```

**404:** `{ "error": "not found" }`  
**400:** invalid address  

Does not require auth.

### `POST /v1/contract/{addr}`

Body JSON:

```json
{
  "name": "optional",
  "abi": [ … ],
  "source": "optional",
  "compiler": "optional"
}
```

- `abi` **required**.  
- On success **200** with same shape as GET.  
- **401** if token configured and missing/wrong.  
- **413** / **400** on oversized or invalid body.  
- **429** when rate-limited.  

### Path routing note

Register before or alongside `/v1/address/` so `/v1/contract/` is not swallowed. Prefer explicit mux route `HandleFunc("/v1/contract/", …)`.

## Explorer UI

### Prerequisites

`PUBLIC_INDEXER_URL` set (same as P1e). When unset: Contract tab remains bytecode-only; no register form.

### Address page — Contract tab (contracts with code)

1. Fetch `GET /v1/contract/{addr}` in parallel with code.  
2. If **registered**:  
   - Badge **ABI registered** (not “Verified”).  
   - Optional name / compiler line.  
   - Collapsible **ABI** (pretty JSON or method/event list from ABI).  
   - Collapsible **Source** if non-empty (`<pre>`).  
   - Keep **Bytecode** section (existing).  
3. If **not found** and indexer up: show **Register ABI** form (name, ABI textarea JSON, optional source, optional compiler) → POST → invalidate query.  
4. If indexer error: soft message; do not break page.

### Tx page (stretch if cheap)

If `to` (or `contractAddress` on create) has registered ABI, resolve function selector against ABI `type:function` entries for a human name; fallback to existing `METHOD_SELECTORS`. **Do in same PR if small; else follow-up.**

### Client

Extend `explorer/src/lib/indexer.ts`:

- `fetchContractMeta(addr)`  
- `registerContract(addr, body)`  
- Types for response  

## Ops / deploy

| Item | Change |
| :--- | :--- |
| `docs/product/indexer.md` | Document contract endpoints + env |
| `docs/product/block-explorer.md` | P1f surface + badge semantics |
| `docs/product/upgrades.md` | P1f **Shipped** when done |
| `agents/debt.md` | Close P1f residual |
| Compose / nginx | Ensure POST to indexer path (edge currently strips prefix for `/indexer/`) — allow POST, not only GET |
| Env | Optional `INDEXER_VERIFY_TOKEN` on indexer container |

No change to node binary or genesis.

## Testing

**Indexer (Go):**

1. Open store → POST contract → GET returns ABI/name.  
2. Upsert overwrites `abi_json` / `updated_at`.  
3. Invalid ABI (not array) → 400.  
4. With token set → POST without token 401; with token 200.  
5. Rate limit: exceed window → 429 (unit-level with injectable clock if needed; else simple counter test).  

**Explorer:** manual or light component not required in Go CI; typecheck via existing `pnpm` if project has explorer test/lint script — run `pnpm --dir explorer build` when touching UI.

## Acceptance

- [ ] SQLite `contracts` + GET/POST `/v1/contract/{addr}`  
- [ ] Open POST + optional token + body/rate limits  
- [ ] Explorer Contract tab: badge, ABI, source, register form when indexer configured  
- [ ] Docs: badge = **registered**, not bytecode-verified  
- [ ] CORS/nginx support POST for indexer edge  
- [ ] Debt + upgrades P1f closed  
- [ ] No wire/consensus change  

## Risks

| Risk | Mitigation |
| :--- | :--- |
| Spam / garbage ABI on public Path B | Rate limit + optional token; last-write-wins; operator can clear DB |
| Users confuse “registered” with audit | UI + docs wording only “ABI registered” |
| Large source pastes | 512 KiB body cap |
| Edge blocks POST | Update nginx allow POST on indexer location |

## Related

- [History indexer](../../product/indexer.md)  
- [Block explorer](../../product/block-explorer.md)  
- [Product upgrades](../../product/upgrades.md)  
- [agents/debt.md](../../../agents/debt.md)  
