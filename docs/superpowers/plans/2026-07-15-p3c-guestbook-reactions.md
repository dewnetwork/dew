# P3c Guestbook Reactions & Replies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship on-chain reactions (4 kinds, toggle) and replies on Guestbook, with SPA UI and Foundry/Hardhat parity. Requires **new contract deploy** (ABI break on `getEntry` / `Signed`).

**Architecture:** Extend `Guestbook.sol` storage with `parentId` and reaction maps; SPA loads counts, `react`/`reply` via MetaMask; docs note operator redeploy for Path B.

**Tech Stack:** Solidity 0.8.24, Foundry tests, Hardhat twin, React + ethers guestbook-web.

**Spec:** [2026-07-15-p3c-guestbook-reactions-design.md](../specs/2026-07-15-p3c-guestbook-reactions-design.md)

---

## File map

| File | Role |
| :--- | :--- |
| `examples/foundry/src/Guestbook.sol` | P3c contract |
| `examples/hardhat/contracts/Guestbook.sol` | Copy parity |
| `examples/foundry/test/Guestbook.t.sol` | Unit tests |
| `examples/guestbook-web/src/config.ts` | ABI + kind constants |
| `examples/guestbook-web/src/rpc.ts` | fetch entries + reactions |
| `examples/guestbook-web/src/wallet.ts` | react / reply txs |
| `examples/guestbook-web/src/App.tsx` | UI chips, reply, thread |
| `docs/product/guestbook.md`, `upgrades.md`, `agents/debt.md` | Ship notes |

---

### Task 1: Solidity + Foundry tests

**Files:**
- Modify: `examples/foundry/src/Guestbook.sol`
- Modify: `examples/foundry/test/Guestbook.t.sol`
- Modify: `examples/hardhat/contracts/Guestbook.sol` (same body)

- [ ] **Step 1: Update tests first (expect compile fail)**

Replace/extend `Guestbook.t.sol`:

```solidity
function test_signAndRead() public {
    vm.prank(alice);
    uint256 id = book.sign("hello dew");
    (address author, uint64 ts, string memory msg_, uint256 parent) = book.getEntry(0);
    assertEq(author, alice);
    assertEq(msg_, "hello dew");
    assertEq(parent, book.PARENT_NONE());
    assertGt(uint256(ts), 0);
    assertEq(id, 0);
}

function test_replyAndReactToggle() public {
    vm.prank(alice);
    uint256 root = book.sign("root");
    address bob = address(0xB0B);
    vm.prank(bob);
    uint256 child = book.reply(root, "reply msg");
    (,,, uint256 parent) = book.getEntry(child);
    assertEq(parent, root);

    vm.prank(bob);
    book.react(root, 0);
    assertEq(book.reactionCount(root, 0), 1);
    assertTrue(book.hasReacted(root, bob, 0));

    vm.prank(bob);
    book.react(root, 0); // toggle off
    assertEq(book.reactionCount(root, 0), 0);
    assertFalse(book.hasReacted(root, bob, 0));
}

function test_reactInvalidKind() public {
    vm.prank(alice);
    book.sign("x");
    vm.expectRevert(bytes("kind"));
    book.react(0, 4);
}

function test_replyBadParent() public {
    vm.expectRevert(bytes("parent"));
    book.reply(0, "nope");
}
```

Fix `test_multipleAuthors` to destructure 4 return values.

- [ ] **Step 2: Implement `Guestbook.sol`**

```solidity
// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

contract Guestbook {
    uint256 public constant MAX_MESSAGE_BYTES = 280;
    uint256 public constant PARENT_NONE = type(uint256).max;
    uint8 public constant MAX_REACTION_KIND = 3;

    struct Entry {
        address author;
        uint64 timestamp;
        string message;
        uint256 parentId;
    }

    Entry[] private _entries;
    mapping(uint256 => mapping(uint8 => uint256)) private _reactionCounts;
    mapping(uint256 => mapping(uint8 => mapping(address => bool))) private _reacted;

    event Signed(uint256 indexed id, address indexed author, string message, uint256 parentId);
    event Reacted(uint256 indexed entryId, address indexed reactor, uint8 kind, bool active);

    function totalEntries() external view returns (uint256) {
        return _entries.length;
    }

    function getEntry(uint256 id)
        external
        view
        returns (address author, uint64 timestamp, string memory message, uint256 parentId)
    {
        require(id < _entries.length, "id");
        Entry storage e = _entries[id];
        return (e.author, e.timestamp, e.message, e.parentId);
    }

    function reactionCount(uint256 entryId, uint8 kind) external view returns (uint256) {
        return _reactionCounts[entryId][kind];
    }

    function hasReacted(uint256 entryId, address who, uint8 kind) external view returns (bool) {
        return _reacted[entryId][kind][who];
    }

    function sign(string calldata message) external returns (uint256 id) {
        return _post(PARENT_NONE, message);
    }

    function reply(uint256 parentId, string calldata message) external returns (uint256 id) {
        require(parentId < _entries.length, "parent");
        return _post(parentId, message);
    }

    function react(uint256 entryId, uint8 kind) external {
        require(entryId < _entries.length, "id");
        require(kind <= MAX_REACTION_KIND, "kind");
        bool was = _reacted[entryId][kind][msg.sender];
        if (was) {
            _reacted[entryId][kind][msg.sender] = false;
            _reactionCounts[entryId][kind] -= 1;
            emit Reacted(entryId, msg.sender, kind, false);
        } else {
            _reacted[entryId][kind][msg.sender] = true;
            _reactionCounts[entryId][kind] += 1;
            emit Reacted(entryId, msg.sender, kind, true);
        }
    }

    function _post(uint256 parentId, string calldata message) internal returns (uint256 id) {
        require(bytes(message).length > 0, "empty");
        require(bytes(message).length <= MAX_MESSAGE_BYTES, "too long");
        id = _entries.length;
        _entries.push(Entry({
            author: msg.sender,
            timestamp: uint64(block.timestamp),
            message: message,
            parentId: parentId
        }));
        emit Signed(id, msg.sender, message, parentId);
    }
}
```

Copy identical file to Hardhat path.

- [ ] **Step 3: Run Foundry tests**

```bash
cd examples/foundry && forge test --match-contract GuestbookTest -vv
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add examples/foundry/src/Guestbook.sol examples/foundry/test/Guestbook.t.sol examples/hardhat/contracts/Guestbook.sol
git commit -m "feat(guestbook): P3c reactions and replies on-chain"
```

---

### Task 2: SPA config, rpc, wallet

**Files:**
- Modify: `examples/guestbook-web/src/config.ts`, `rpc.ts`, `wallet.ts`

- [ ] **Step 1: ABI + kinds**

```ts
export const REACTION_KINDS = [
  { kind: 0, emoji: "👍", label: "thumbs up" },
  { kind: 1, emoji: "❤️", label: "heart" },
  { kind: 2, emoji: "🔥", label: "fire" },
  { kind: 3, emoji: "🎉", label: "party" },
] as const;

export const GUESTBOOK_ABI = [
  "function totalEntries() view returns (uint256)",
  "function PARENT_NONE() view returns (uint256)",
  "function getEntry(uint256 id) view returns (address author, uint64 timestamp, string message, uint256 parentId)",
  "function sign(string message) returns (uint256 id)",
  "function reply(uint256 parentId, string message) returns (uint256 id)",
  "function react(uint256 entryId, uint8 kind)",
  "function reactionCount(uint256 entryId, uint8 kind) view returns (uint256)",
  "function hasReacted(uint256 entryId, address who, uint8 kind) view returns (bool)",
] as const;
```

- [ ] **Step 2: `GuestbookEntry` + fetch**

```ts
export type GuestbookEntry = {
  id: number;
  author: string;
  timestamp: number;
  message: string;
  parentId: string; // bigint as decimal string; compare to parentNone
  reactions: number[]; // length 4
  myReactions?: boolean[]; // if account known
};

// In fetchEntries: getEntry 4-tuple; load PARENT_NONE once;
// for each entry, reactionCount for kinds 0..3
// optional hasReacted if account param provided
```

On ABI decode failure, throw clear error: `Guestbook contract is not P3c (redeploy required)`.

- [ ] **Step 3: wallet helpers**

```ts
export async function reactGuestbook(guestbook, account, entryId, kind, chainId): Promise<string>
export async function replyGuestbook(guestbook, account, parentId, message, chainId): Promise<string>
```

Mirror `signGuestbook` pattern (Contract + signer + wait optional; return tx hash).

- [ ] **Step 4: Build**

```bash
pnpm --dir examples/guestbook-web build
```

- [ ] **Step 5: Commit**

```bash
git add examples/guestbook-web/src/config.ts examples/guestbook-web/src/rpc.ts examples/guestbook-web/src/wallet.ts
git commit -m "feat(guestbook-web): P3c ABI client for react and reply"
```

---

### Task 3: App UI — chips, reply, thread

**Files:**
- Modify: `examples/guestbook-web/src/App.tsx`

- [ ] **Step 1: Thread model**

```ts
// parentNone from first load or constant type(uint256).max as string
// roots = entries where parentId === parentNone, newest first
// children(id) = entries where parentId === String(id), oldest first
```

- [ ] **Step 2: Entry card**

- Message, author, ts, explorer links (existing)  
- Reaction row: 4 buttons `emoji count`; highlight if `myReactions[k]`; onClick → `reactGuestbook` then `load()`  
- Reply: toggle form; submit → `replyGuestbook` then `load()`  
- Indent children with `pl-4 border-l`  

- [ ] **Step 3: Compose**

- Top form remains root `sign` + Burst  
- Do not burst replies  

- [ ] **Step 4: Filter**

Keep filter on author/message for all entries; when filtering, still show parent context if child matches (optional: show matching entries flat). **Simple:** filter flat list then render threads only for roots that match OR have matching child.

- [ ] **Step 5: Build**

```bash
pnpm --dir examples/guestbook-web build
```

- [ ] **Step 6: Commit**

```bash
git add examples/guestbook-web/src/App.tsx
git commit -m "feat(guestbook-web): P3c reaction chips and reply threads"
```

---

### Task 4: Docs + debt

**Files:**
- Modify: `docs/product/guestbook.md`, `docs/product/upgrades.md`, `agents/debt.md`, `docs/build/phases.md` / roadmap if Track 1 mentions P3c

- [ ] Document reaction kinds, reply, redeploy Path B (`PUBLIC_GUESTBOOK`)  
- [ ] Mark P3c shipped in upgrades + debt  
- [ ] Note default SPA address may still be pre-P3c until operator redeploys  

```bash
# verify
cd examples/foundry && forge test --match-contract GuestbookTest -vv
pnpm --dir examples/guestbook-web build
```

- [ ] Commit

```bash
git commit -m "docs(product): ship P3c Guestbook reactions and replies"
```

---

## Self-review

| Spec | Task |
| :--- | :--- |
| react toggle 0..3 | 1 |
| reply + parentId | 1 |
| getEntry 4-tuple | 1–2 |
| SPA chips/reply/thread | 3 |
| Burst root only | 3 |
| Docs redeploy | 4 |
| Foundry+Hardhat parity | 1 |

---

## Execution handoff

Plan saved to `docs/superpowers/plans/2026-07-15-p3c-guestbook-reactions.md`.

**1. Subagent-Driven (recommended)** · **2. Inline Execution**

Which approach?
