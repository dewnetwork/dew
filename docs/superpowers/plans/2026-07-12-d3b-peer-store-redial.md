# D3b Peer Store + Auto-Redial Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Persist known peers to `peers.json` and auto-redial after disconnect/restart without manual `RedialMesh`.

**Architecture:** Extend `p2p.PeerStore` with JSON load/save; `Host` owns a maintain/redial loop seeded by bootnodes + disk; wire path via `Stack` / `dew run --datadir`.

**Tech Stack:** Go, existing `p2p` package, no new deps.

**Design spec:** [docs/superpowers/specs/2026-07-12-d3b-peer-store-redial-design.md](../specs/2026-07-12-d3b-peer-store-redial-design.md)

---

## File map

| File | Role |
| :--- | :--- |
| `p2p/store.go` | KnownPeer + ban; persist hooks |
| `p2p/persist.go` | Atomic JSON load/save, eviction helpers |
| `p2p/redial.go` | Backoff state + maintain loop |
| `p2p/host.go` | Config fields; Start/Close/onPeerClosed wire |
| `p2p/store_persist_test.go` | Unit tests |
| `p2p/redial_test.go` | Auto-redial integration |
| `node/stack.go` | PeerStorePath on StackConfig → Host |
| `cmd/dew/main.go` | `--datadir` → peers.json |
| `docs/development/private-testnet.md` | Data dir layout |
| `docs/development/d3-scale.md` / phases / debt | D3b status |

---

### Task 1: Persistent PeerStore

**Files:** `p2p/store.go`, `p2p/persist.go`, `p2p/store_persist_test.go`

- [ ] KnownPeer gains `BanScore int`
- [ ] `LoadFromFile` / `SaveToFile` (atomic rename)
- [ ] `EvictOlderThan(d time.Duration)` skip bootnode IDs optional set
- [ ] `BanDialThreshold = 100`; dial helpers check score
- [ ] Tests: round-trip, eviction, corrupt file handling

### Task 2: Redial loop on Host

**Files:** `p2p/redial.go`, `p2p/host.go`, `p2p/redial_test.go`

- [ ] Config: `PeerStorePath`, `Bootnodes`, `Redial bool`
- [ ] Start: load file, seed bootnodes, start goroutine
- [ ] onPeerClosed: schedule redial; do not Forget
- [ ] Backoff 1s…5m per addr
- [ ] Test: A↔B connect, close B peer side / kill conn, A re-establishes within 30s without Dial helper

### Task 3: Wire operators

**Files:** `node/stack.go`, `cmd/dew/main.go`

- [ ] `StackConfig.PeerStorePath` / `DataDir`
- [ ] `dew run --datadir DIR` → `DIR/peers.json`
- [ ] Pass `Bootnodes` into `p2p.Config` for continuous redial

### Task 4: Docs + debt

- [ ] private-testnet data dir
- [ ] d3-scale / phases / debt D3b checked
- [ ] `go test ./p2p/ ./node/ ./devnet/ -count=1`

---
