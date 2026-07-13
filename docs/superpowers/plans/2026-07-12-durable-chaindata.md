# Durable chaindata Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox syntax for tracking. Do not mark complete without running the listed tests.

**Goal:** Persist canonical chain + flat state under `<datadir>/chaindata` (Pebble) so process restart recovers tip, balances, receipts, and tx lookups. Keep `peers.json` separate (D3b).

**Architecture:** Implement `db.PebbleDB` behind existing `db.Database` / `Batcher` / `IteratePrefix`. Add `node.Open` + write-through on seal/import + hydrate on start. Wire `dew run --datadir` to open chaindata. Atomic batch for state+chain+tip.

**Tech stack:** Go 1.23, `github.com/cockroachdb/pebble` (direct require), existing RLP/type codecs.

**Design spec:** [docs/superpowers/specs/2026-07-12-durable-chaindata-design.md](../specs/2026-07-12-durable-chaindata-design.md)  
**Dev overview:** [docs/development/durable-chaindata.md](../../development/durable-chaindata.md)

---

## File map

| File | Role |
| :--- | :--- |
| `db/pebble.go` | Pebble open/close; Database + Batcher + IteratePrefix |
| `db/pebble_test.go` | Disk round-trip, batch atomicity, reopen |
| `go.mod` / `go.sum` | Direct `pebble` require |
| `node/store.go` (or `node/chaindb.go`) | Key schema helpers; encode/decode tip, header, body, receipt, tx index |
| `node/node.go` | `Open`; datadir field; Close DB; hydrate |
| `node/build.go` / `node/import.go` | Persist after successful seal / import |
| `node/persist_test.go` | Temp dir: mine/import → reopen → assert |
| `core/state/statedb.go` | Optional: batch-aware Commit for true atomicity |
| `cmd/dew/main.go` | Wire `--datadir` → node chain open |
| `deploy/docker-compose.yml` / node compose | Volume for chaindata |
| `docs/development/private-testnet.md` | Datadir layout update |
| `docs/development/d3-scale.md` / `phases.md` | Pointer + ordering |
| `agents/debt.md` | Track epic + residuals |

---

### Task 1: Pebble backend

**Files:** `db/pebble.go`, `db/pebble_test.go`, `go.mod`

- [x] `OpenPebble(path string) (*PebbleDB, error)` creates dir if needed
- [x] Implement `Has` / `Get` / `Put` / `Delete` / `Close`
- [x] `NewBatch` → `Write` / `Reset`; values copied as needed (no alias after Write)
- [x] `IteratePrefix(prefix, fn)` ordered walk
- [x] Tests on `t.TempDir()`: round-trip, overwrite, delete, batch, reopen after Close
- [x] `go test ./db/ -count=1`

**Exit:** MemoryDB unchanged; Pebble passes same behavioral expectations as memory tests where applicable.

---

### Task 2: Chain key schema + encode helpers

**Files:** `node/store.go` (name flexible), unit tests next to it

- [x] Constants for prefixes: `H`, `B`, `N`, `R`, `T`, `meta/…` per design
- [x] Helpers: `headerKey`, `bodyKey`, `canonicalKey`, `receiptKey`, `txLookupKey`, `tipKey`
- [x] Encode/decode tip (`height`, `hash`)
- [x] Write/read header, body, canonical, receipt, tx lookup via `db.Database` / batch
- [x] Covered via restart + encode paths in `node` tests

**Exit:** Pure helpers tested without full node if possible.

---

### Task 3: Atomic persist on seal / import

**Files:** `node/build.go`, `node/import.go`, `node/node.go`, maybe `core/state`

- [x] After successful block apply, build one batch:
  - state dirty keys (`StateDB.FlushDirtyTo` + `ClearDirty` after Write)
  - `H`/`B`/`N`/`R`/`T` + `meta/tip`
- [x] On batch error: do not advance in-memory tip; RevertToSnapshot on auto-mine/import
- [x] Auto-mine path and `ImportCommittedBlock` both call `persistBlockLocked`
- [x] Tests: `TestNode_RestartRecoversTip`

**Exit:** With Pebble temp dir, kill process simulation via Close + Open sees data only after successful Write.

---

### Task 4: `Open` + hydrate + CLI

**Files:** `node/node.go`, `cmd/dew/main.go`, `cmd/dew/bft.go` if Stack constructs node

- [x] `Open(genesis, chainDataDir string) (*Node, error)`
  - open Pebble
  - if empty: commit genesis, write meta (version, chainId, genesisHash), tip=0
  - if non-empty: verify genesis hash + chainId; load tip; hydrate `0..tip` into maps; rebuild logs
- [x] Mismatch genesis → error (no silent wipe)
- [x] `dew run --datadir DIR`:
  - peers: `DIR/peers.json` (existing)
  - chain: `DIR/chaindata`
- [x] Without `--datadir`: MemoryDB (existing)
- [x] `Node.Close` / process shutdown closes Pebble
- [x] Test: `TestNode_RestartRecoversTip`

**Exit:** Manual smoke:

```bash
go build -o bin/dew ./cmd/dew
mkdir -p /tmp/dew-data
./bin/dew run --datadir /tmp/dew-data --genesis genesis.json &
# send tx or use auto-mine path; note eth_blockNumber
kill %1
./bin/dew run --datadir /tmp/dew-data --genesis genesis.json
# eth_blockNumber >= previous
```

---

### Task 5: Deploy + docs + debt

**Files:** `deploy/*`, `docs/development/*`, `agents/debt.md`, phases/d3-scale as needed

- [x] Path B compose: volume mount `…:/var/lib/dew` and pass `--datadir /var/lib/dew`
- [x] Compose `multi`: each validator already has volume + `--datadir` (chaindata now used)
- [x] Update [private-testnet.md](../../development/private-testnet.md) datadir table (`chaindata/`)
- [x] Mark design status / checklist progress; link from [d3-scale.md](../../development/d3-scale.md)
- [x] `agents/debt.md`: durable chaindata checked; peers remain separate
- [x] `go test ./... -count=1`

**Exit:** Docs match behavior; operator can restart container without tip reset when volume present.

---

### Task 6 (optional follow-up): Lazy hydrate

- [ ] Load tip header only; lazy `GetBlock` from `H`/`B`
- [ ] Only if full hydrate becomes slow in soak

---

## Acceptance checklist (epic done)

- [x] Restart with same `--datadir` recovers tip + state + receipt + tx hash lookup
- [x] Genesis mismatch refuses start
- [x] No `--datadir` still MemoryDB; default unit tests green without disk engines flaking CI
- [x] Peers **not** stored in Pebble
- [x] `go test ./...` pass (heavy integration still optional env)
- [x] Docs: design + this plan + private-testnet layout + debt pointer

---

## Out of order / do not do

- Do not put peer records into Pebble
- Do not invent a custom storage engine
- Do not change DewTx / precompile / fee freeze surfaces
- Do not implement pruning or snap sync in this plan
