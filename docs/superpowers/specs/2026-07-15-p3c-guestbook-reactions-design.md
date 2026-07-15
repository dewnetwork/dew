# P3c — Guestbook reactions & replies (design)

**Status:** Approved design (2026-07-15)  
**Track:** 1 (product / demo)  
**Freeze:** `public-testnet-v1` — ordinary EVM contract + SPA only; no consensus/wire change  
**Parent:** [guestbook.md](../../product/guestbook.md) · [upgrades — P3c](../../product/upgrades.md)

## Problem

Guestbook today is append-only `sign(message)`. Product backlog defers **reactions / replies**, which need **new contract methods** and a **redeploy** (new address). Path B live address must be updated by operator when redeployed.

## Goals

1. On-chain **emoji reactions** (fixed kind set) with per-entry counts and toggle.  
2. On-chain **replies** to an existing entry (parent id + message).  
3. SPA shows counts, react buttons, reply compose, simple thread indent.  
4. Keep Foundry + Hardhat contract parity and existing filter / Mine / `?author=` / Burst.  
5. Document redeploy + env update; do not claim old contract supports P3c.

## Non-goals

- Nested reply depth > 1 level (replies to replies allowed if parent exists, but UI may only indent one level).  
- Edit/delete messages.  
- Custom emoji strings on-chain.  
- Indexer / full-text search.  
- Auto-updating live Path B contract address without operator deploy.  
- Wire freeze / DewTx / precompiles.

## Approach (locked): A — React + reply-to

### Constants

| Item | Value |
| :--- | :--- |
| `MAX_MESSAGE_BYTES` | 280 (unchanged) |
| Reaction kinds | `uint8` **0..3** only |
| Kind UI map | 0 👍 · 1 ❤️ · 2 🔥 · 3 🎉 |
| Root parent | `type(uint256).max` (`PARENT_NONE`) |

### Storage

```solidity
struct Entry {
    address author;
    uint64 timestamp;
    string message;
    uint256 parentId; // PARENT_NONE if root
}

Entry[] private _entries;

// entryId => kind => count
mapping(uint256 => mapping(uint8 => uint256)) private _reactionCounts;
// entryId => kind => reactor => reacted
mapping(uint256 => mapping(uint8 => mapping(address => bool))) private _reacted;
```

### Functions

| Method | Behavior |
| :--- | :--- |
| `sign(string message) → id` | Root entry; `parentId = PARENT_NONE`; same length checks as today |
| `reply(uint256 parentId, string message) → id` | Require `parentId < length`; same message checks; set `parentId` |
| `react(uint256 entryId, uint8 kind)` | Require entry exists; `kind <= 3`; **toggle**: if already reacted for kind, clear and decrement count; else set and increment |
| `totalEntries()` | unchanged |
| `getEntry(id)` | returns `(author, timestamp, message, parentId)` — **ABI break** vs old |
| `reactionCount(entryId, kind)` | view |
| `hasReacted(entryId, who, kind)` | view |
| `PARENT_NONE()` or public constant | so clients can detect roots |

### Events

```solidity
event Signed(uint256 indexed id, address indexed author, string message, uint256 parentId);
event Reacted(uint256 indexed entryId, address indexed reactor, uint8 kind, bool active);
```

`Signed` gains `parentId` (breaking for old indexers of the old contract — new deploy only).

### SPA (`examples/guestbook-web`)

- Extend ABI + types with `parentId`, `reactionCount`, `hasReacted`, `react`, `reply`.  
- Feed: load entries; group children under parents for display (roots sorted newest-first; replies under parent, oldest-first within parent).  
- Per entry: four reaction chips with counts; connected wallet calls `react`; refresh counts after receipt.  
- Reply: expand compose under entry → `reply(parentId, text)`.  
- Burst ×2 remains **root `sign` only**.  
- Filter / Mine / `?author=` apply to visible entries (message + author); replies still match author filter.

### Solidity locations (parity)

| Path | Action |
| :--- | :--- |
| `examples/foundry/src/Guestbook.sol` | Implement P3c |
| `examples/hardhat/contracts/Guestbook.sol` | Same source |
| Foundry tests if any | Extend / add |
| Deploy scripts | Still `new Guestbook()`; note P3c in comments |

### Docs / ops

- [guestbook.md](../../product/guestbook.md) — reactions UI + redeploy note  
- [upgrades.md](../../product/upgrades.md) — P3c shipped when code+docs land  
- [agents/debt.md](../../../agents/debt.md) — close P3c residual  
- Path B: operator redeploy + set `PUBLIC_GUESTBOOK` (document; no secret keys in repo)  
- Default `PUBLIC_GUESTBOOK` in SPA may stay old address until operator updates — SPA must work against **old** contract only if we detect missing methods, **or** require new address.  

**Client compatibility (locked):** SPA targets **P3c ABI only**. Document that old contract addresses need redeploy. Optional soft error if `getEntry` fails ABI decode (“contract not P3c — redeploy”).

## Testing

1. Foundry (or Hardhat) unit: sign → reply → react toggle → counts.  
2. SPA: typecheck/build.  
3. Manual local: deploy + set env + react/reply flow.

## Acceptance

- [ ] Foundry + Hardhat Guestbook with react + reply + getEntry parentId  
- [ ] SPA: chips, counts, reply, thread display  
- [ ] Docs + debt + upgrades P3c  
- [ ] No wire/consensus change  
- [ ] Live Path B address update is **operator** step (documented)

## Risks

| Risk | Mitigation |
| :--- | :--- |
| Path B still points at old contract | Clear docs + SPA error if methods missing |
| Gas cost of nested maps | Demo-scale only |
| Event ABI break | New contract only |

## Related

- [Guestbook product](../../product/guestbook.md)  
- [upgrades Track 1](../../product/upgrades.md)  
- Foundry `Guestbook.sol` · Hardhat twin · `guestbook-web`  
