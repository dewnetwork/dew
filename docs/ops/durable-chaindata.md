---
title: Durable chaindata
description: Disk persistence plan for chain + state under datadir (Pebble); peers stay in peers.json.
category: ops
order: 36
status: stable
---

# Durable chaindata

**Status:** implemented (July 2026).  
**Freeze:** does not change `public-testnet-v1` wire formats.  

See also [D3 scale](../scale/d3-scale.md).

## Why

Before this work the node used **in-memory** state and chain indexes (`db.MemoryDB` + RAM maps). `--datadir` only persisted **P2P** peers (`peers.json`, D3b). Process or container restart **lost tip and balances**.

## Decisions (summary)

| Topic | Decision |
| :--- | :--- |
| Engine | **Pebble only** (no MemoryDB backend) |
| Schema | **Dew-owned** key layout; state prefixes `a`/`s`/`c` unchanged |
| Peers | **Remain** `<datadir>/peers.json` — not inside chain DB |
| Atomicity | One batch per sealed/imported block: state + headers/bodies + indexes + tip |
| Default CLI | `--datadir` defaults to **`/var/lib/dew`** (same as Docker volume mount) |

## Target datadir layout

```text
<datadir>/
  chaindata/     # Pebble — blocks, receipts, tx index, flat state, tip meta
  peers.json     # D3b P2P known peers only
```

| Flag | Chain | Peers |
| :--- | :--- | :--- |
| `--datadir DIR` (default **`/var/lib/dew`**) | `DIR/chaindata` | `DIR/peers.json` when P2P enabled |
| Tests | `db.OpenTest` / `node.OpenTest` → temp Pebble | n/a |

## Acceptance

1. Mine or import blocks with `--datadir`, stop process, start again with same dir + genesis → tip and state match (`go test ./node/ -run RestartRecoversTip`).
2. Genesis hash / chain ID mismatch on open → **refuse** start (no silent re-genesis).
3. `go test ./...` uses temp Pebble (`db.OpenTest` / `node.OpenTest`); no in-memory DB backend.
4. Compose Path B / multi volume-mount `/var/lib/dew` (default datadir — no `--datadir` flag required).

## Packages

| Package | Role |
| :--- | :--- |
| `db/pebble.go` | Pebble backend |
| `node/open.go` | `Open`, hydrate, `Close`, `ChainDataDir` |
| `node/store.go` / `persist.go` | Key schema + atomic batch on seal/import |
| `core/state` | `FlushDirtyTo` / `ClearDirty` for batch commit |
| `cmd/dew` | `--datadir` → `chaindata/` + `peers.json` |

## Ordering

```text
D3a multiproc BFT (done) + D3b peers.json (done)
    → durable chaindata (this)
    → D3d Path A / meaningful D3c staking on long-lived nets
```

See [D3 scale](../scale/d3-scale.md) and [agents/debt.md](../../agents/debt.md).

## Non-goals

Custom storage engine; peer data in Pebble; snap sync; pruning/freezer; PE multi-version store upgrade.
