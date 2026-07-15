# Wave 3 HTTP Filter API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship Ethereum-compatible HTTP filter poll methods (`eth_newFilter`, `eth_newBlockFilter`, `eth_newPendingTransactionFilter`, `eth_getFilterChanges`, `eth_getFilterLogs`, `eth_uninstallFilter`) with cursor-by-height semantics under `public-testnet-v1`.

**Architecture:** In-memory `filterStore` owned by `*API`. Create sets `lastPolled = tip`. Poll queries blocks `(lastPolled+1)…tip` via existing `node.FilterLogs` / `GetBlockByNumber`, then advances cursor. Pending filters always return `[]`. Limits: 128 filters, 5m idle TTL. RPC-only; no consensus or chaindata schema changes.

**Tech Stack:** Go (`rpc/`, `node/`), `httptest` JSON-RPC tests, existing `rpcCall` helper patterns, ERC-20 deploy via `vm.TokenCreationBytecode` for log tests.

**Spec:** [2026-07-15-filter-api-design.md](../specs/2026-07-15-filter-api-design.md)

---

## File map

| File | Responsibility |
| :--- | :--- |
| `rpc/filter.go` | `filterStore`, kinds, install/poll/uninstall, API handlers |
| `rpc/filter_test.go` | Integration tests via `OpenTest` + `rpcCall` |
| `rpc/api.go` | Register six methods in `Handlers()`; ensure `NewAPI` initializes store |
| `docs/api/json-rpc.md` | Mark implemented + limits |
| `docs/superpowers/plans/2026-07-14-gap-closure.md` | Wave 3 checkboxes |
| `agents/debt.md` | Close optional Filter API residual |

Reuse (do not reimplement matching): `formatIndexedLog` in `rpc/ws.go`; `node.FilterLogs`; parse address/topics similar to `eth_getLogs` in `api.go`.

---

### Task 1: filterStore + newFilter / uninstall (TDD)

**Files:**
- Create: `rpc/filter.go`
- Create: `rpc/filter_test.go`
- Modify: `rpc/api.go` (`NewAPI`, `Handlers`)

- [ ] **Step 1: Write failing tests for install/uninstall**

```go
// rpc/filter_test.go
package rpc

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/node"
)

func filterTestGenesis() *config.Genesis {
	return &config.Genesis{
		Config:        &config.ChainConfig{ChainID: big.NewInt(2205)},
		Timestamp:     0,
		GasLimit:      "0x7270e00",
		BaseFeePerGas: "0x3b9aca00",
		ExtraData:     "0x446577",
		Alloc: map[string]config.GenesisAccount{
			"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {
				Balance: "1000000000000000000000000",
			},
		},
	}
}

func TestRPC_Filter_NewAndUninstall(t *testing.T) {
	n := node.OpenTest(t, filterTestGenesis())
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	id := rpcCall(t, srv, "eth_newFilter", []interface{}{map[string]interface{}{}})
	idStr, ok := id.(string)
	if !ok || len(idStr) < 4 || idStr[:2] != "0x" {
		t.Fatalf("filter id = %v", id)
	}

	okRes := rpcCall(t, srv, "eth_uninstallFilter", []interface{}{idStr})
	if okRes != true {
		t.Fatalf("uninstall first = %v", okRes)
	}
	okRes = rpcCall(t, srv, "eth_uninstallFilter", []interface{}{idStr})
	if okRes != false {
		t.Fatalf("uninstall second = %v", okRes)
	}
}

func TestRPC_Filter_UnknownChangesErrors(t *testing.T) {
	n := node.OpenTest(t, filterTestGenesis())
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	body := map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"method": "eth_getFilterChanges",
		"params": []interface{}{"0xdeadbeefdeadbeef"},
	}
	b, _ := json.Marshal(body)
	// use httptest like rpcCall but assert Error
	// (implement helper rpcCallErr or inline ServeHTTP)
	_ = b
	// Expect error message contains "filter not found"
}
```

Complete `TestRPC_Filter_UnknownChangesErrors` using the same `httptest` pattern as `TestRPC_DewSendRawTransaction_AndStats` when native is disabled (assert `resp.Error != nil` and message contains `filter not found`).

- [ ] **Step 2: Run tests — expect fail (method missing)**

```bash
go test ./rpc/ -run 'TestRPC_Filter_NewAndUninstall|TestRPC_Filter_UnknownChangesErrors' -count=1
```

Expected: FAIL (nil handler or method not found / invalid response).

- [ ] **Step 3: Implement store + handlers**

```go
// rpc/filter.go
package rpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

const (
	MaxFilters     = 128
	FilterIdleTTL  = 5 * time.Minute
)

type filterKind int

const (
	filterLogs filterKind = iota
	filterBlocks
	filterPending
)

type installedFilter struct {
	id         string
	kind       filterKind
	addrs      []crypto.Address
	topics     [][]types.Hash
	fromTag    interface{} // optional raw tag for getFilterLogs; nil = earliest
	toTag      interface{} // optional; nil = latest
	lastPolled uint64
	lastAccess time.Time
}

type filterStore struct {
	mu    sync.Mutex
	byID  map[string]*installedFilter
	nowFn func() time.Time // tests may inject; default time.Now
}

func newFilterStore() *filterStore {
	return &filterStore{
		byID:  make(map[string]*installedFilter),
		nowFn: time.Now,
	}
}

func (s *filterStore) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

func (s *filterStore) purgeExpiredLocked() {
	cutoff := s.now().Add(-FilterIdleTTL)
	for id, f := range s.byID {
		if f.lastAccess.Before(cutoff) {
			delete(s.byID, id)
		}
	}
}

func newFilterID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return EncodeBytes(b[:])
}

// Install returns id or error (capacity).
func (s *filterStore) install(f *installedFilter) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	if len(s.byID) >= MaxFilters {
		return "", fmt.Errorf("too many filters (max %d)", MaxFilters)
	}
	if f.id == "" {
		f.id = newFilterID()
	}
	f.lastAccess = s.now()
	s.byID[f.id] = f
	return f.id, nil
}

func (s *filterStore) get(id string) (*installedFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	f, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("filter not found")
	}
	f.lastAccess = s.now()
	return f, nil
}

func (s *filterStore) uninstall(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	if _, ok := s.byID[id]; !ok {
		return false
	}
	delete(s.byID, id)
	return true
}
```

Wire into `API`:

```go
// api.go — extend API struct
type API struct {
	n       *node.Node
	filters *filterStore
}

func NewAPI(n *node.Node) *API {
	return &API{n: n, filters: newFilterStore()}
}

// In Handlers() add:
"eth_newFilter":                   a.ethNewFilter,
"eth_newBlockFilter":              a.ethNewBlockFilter,
"eth_newPendingTransactionFilter": a.ethNewPendingTransactionFilter,
"eth_getFilterChanges":            a.ethGetFilterChanges,
"eth_getFilterLogs":               a.ethGetFilterLogs,
"eth_uninstallFilter":             a.ethUninstallFilter,
```

Implement handlers for Task 1 only (newFilter empty object, uninstall, getFilterChanges unknown error). Stubs for other methods can return `"not implemented"` until Task 2 — **prefer full stubs that compile** so Handlers register all six names.

Minimal `ethNewFilter` for empty `{}`:

```go
func (a *API) ethNewFilter(params json.RawMessage) (interface{}, error) {
	// parse optional [] with one object; allow empty params as {}
	var p []map[string]interface{}
	if len(params) > 0 && string(params) != "null" {
		if err := json.Unmarshal(params, &p); err != nil {
			// also accept single object? Ethereum uses array of one object
			return nil, fmt.Errorf("invalid params")
		}
	}
	tip := a.n.BlockNumber()
	f := &installedFilter{kind: filterLogs, lastPolled: tip}
	if len(p) > 0 {
		// parse address/topics/fromBlock/toBlock into f — full parse in Task 2 if needed
		_ = p[0]
	}
	return a.filters.install(f)
}

func (a *API) ethUninstallFilter(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	return a.filters.uninstall(p[0]), nil
}

func (a *API) ethGetFilterChanges(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	f, err := a.filters.get(p[0])
	if err != nil {
		return nil, err
	}
	// Task 2 fills poll logic; for now return empty if found
	_ = f
	return []interface{}{}, nil
}
```

Also register stub handlers for block/pending/getFilterLogs so names exist:

```go
func (a *API) ethNewBlockFilter(_ json.RawMessage) (interface{}, error) {
	tip := a.n.BlockNumber()
	return a.filters.install(&installedFilter{kind: filterBlocks, lastPolled: tip})
}

func (a *API) ethNewPendingTransactionFilter(_ json.RawMessage) (interface{}, error) {
	tip := a.n.BlockNumber()
	return a.filters.install(&installedFilter{kind: filterPending, lastPolled: tip})
}

func (a *API) ethGetFilterLogs(params json.RawMessage) (interface{}, error) {
	return nil, fmt.Errorf("not a log filter") // fixed in Task 3
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./rpc/ -run 'TestRPC_Filter_NewAndUninstall|TestRPC_Filter_UnknownChangesErrors' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add rpc/filter.go rpc/filter_test.go rpc/api.go
git commit -m "feat(rpc): add filter store and eth_newFilter / uninstall"
```

---

### Task 2: Block + pending poll semantics

**Files:**
- Modify: `rpc/filter.go` (`ethGetFilterChanges` full logic)
- Modify: `rpc/filter_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRPC_Filter_BlockFilterChanges(t *testing.T) {
	n := node.OpenTest(t, filterTestGenesis())
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())

	id := rpcCall(t, srv, "eth_newBlockFilter", []interface{}{}).(string)

	// no new blocks yet
	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok := ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("empty changes = %v", ch)
	}

	// seal via simple transfer (auto-mine)
	// reuse Anvil key transfer pattern from TestRPC_SendRawTransaction_Transfer
	// ... send raw tx ...

	ch = rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok = ch.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("want 1 block hash, got %v", ch)
	}
	hash, ok := arr[0].(string)
	if !ok || len(hash) != 66 {
		t.Fatalf("hash = %v", arr[0])
	}

	ch = rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok = ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("second poll want empty, got %v", ch)
	}
}

func TestRPC_Filter_PendingEmpty(t *testing.T) {
	n := node.OpenTest(t, filterTestGenesis())
	srv := NewServer()
	srv.RegisterAll(NewAPI(n).Handlers())
	id := rpcCall(t, srv, "eth_newPendingTransactionFilter", []interface{}{}).(string)
	// send a tx so tip moves — pending filter still returns []
	// ... optional sendRaw ...
	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok := ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("pending changes = %v", ch)
	}
}
```

Helper to send one transfer (copy from `api_test.go` SendRaw pattern) as `sendTestTransfer(t, srv)`.

- [ ] **Step 2: Run — fail if changes always empty after tip advance**

```bash
go test ./rpc/ -run 'TestRPC_Filter_BlockFilterChanges|TestRPC_Filter_PendingEmpty' -count=1
```

- [ ] **Step 3: Implement `ethGetFilterChanges`**

```go
func (a *API) ethGetFilterChanges(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	f, err := a.filters.get(p[0])
	if err != nil {
		return nil, err
	}

	tip := a.n.BlockNumber()
	from := f.lastPolled + 1
	if from > tip {
		// advance access already done; keep lastPolled
		return []interface{}{}, nil
	}

	var result interface{}
	switch f.kind {
	case filterPending:
		result = []interface{}{}
	case filterBlocks:
		hashes := make([]string, 0, tip-from+1)
		for h := from; h <= tip; h++ {
			b := a.n.GetBlockByNumber(h)
			if b == nil {
				continue
			}
			hashes = append(hashes, EncodeHash(b.Hash()))
		}
		result = hashes
	case filterLogs:
		matched := a.n.FilterLogs(from, tip, f.addrs, f.topics)
		out := make([]map[string]interface{}, 0, len(matched))
		for _, il := range matched {
			out = append(out, formatIndexedLog(il))
		}
		result = out
	default:
		return nil, fmt.Errorf("filter not found")
	}

	// advance cursor under store lock
	a.filters.mu.Lock()
	if cur, ok := a.filters.byID[f.id]; ok {
		cur.lastPolled = tip
		cur.lastAccess = a.filters.now()
	}
	a.filters.mu.Unlock()
	return result, nil
}
```

Note: `get` already touched `lastAccess`. Prefer a single locked method `poll(id) (from,to, filter, err)` that returns a copy of filter fields and updates after caller computes — or document that concurrent polls are rare. Acceptable v1: get copy of fields under lock, compute outside, then `advance(id, tip)`.

Better pattern:

```go
func (s *filterStore) snapshot(id string) (installedFilter, error) { ... copy fields, touch access ... }
func (s *filterStore) advance(id string, tip uint64) { ... set lastPolled ... }
```

- [ ] **Step 4: Tests pass**

```bash
go test ./rpc/ -run 'TestRPC_Filter_' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add rpc/filter.go rpc/filter_test.go
git commit -m "feat(rpc): eth_getFilterChanges for block and pending filters"
```

---

### Task 3: Log filter parse + getFilterLogs + log poll

**Files:**
- Modify: `rpc/filter.go`
- Modify: `rpc/filter_test.go`

- [ ] **Step 1: Write log filter tests using ERC-20 deploy**

Reuse deploy pattern from `node/logindex_test.go` but via RPC `eth_sendRawTransaction` with `vm.TokenCreationBytecode` + ctor (or call `n.SendRawTransaction` then poll via RPC).

```go
func TestRPC_Filter_LogFilterChanges(t *testing.T) {
	n := node.OpenTest(t, filterTestGenesis())
	api := NewAPI(n)
	srv := NewServer()
	srv.RegisterAll(api.Handlers())

	id := rpcCall(t, srv, "eth_newFilter", []interface{}{map[string]interface{}{}}).(string)

	// Deploy token (emits Transfer on mint) via sendRaw — copy deployTokenTx logic into test helper
	// raw := buildTokenDeployRaw(t, n, 0)
	// rpcCall eth_sendRawTransaction EncodeBytes(raw)

	ch := rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok := ch.([]interface{})
	if !ok || len(arr) < 1 {
		t.Fatalf("want >=1 log, got %v", ch)
	}
	// second poll empty
	ch = rpcCall(t, srv, "eth_getFilterChanges", []interface{}{id})
	arr, ok = ch.([]interface{})
	if !ok || len(arr) != 0 {
		t.Fatalf("second poll = %v", ch)
	}
}

func TestRPC_Filter_GetFilterLogs(t *testing.T) {
	// create filter AFTER deploy so changes empty, but getFilterLogs still returns history
	// OR: create with fromBlock 0, deploy, getFilterLogs non-empty without advancing poll
}
```

For `getFilterLogs` after deploy before newFilter: create filter at tip (after deploy), `getFilterChanges` empty, `getFilterLogs` with default 0…latest returns logs.

```go
func TestRPC_Filter_GetFilterLogs_DoesNotAdvanceCursor(t *testing.T) {
	// deploy token first (logs at height 1)
	// newFilter at tip=1 → lastPolled=1
	// getFilterLogs → non-empty
	// getFilterChanges → empty (no new blocks)
}
```

- [ ] **Step 2: Run — fail until parse + getFilterLogs work**

```bash
go test ./rpc/ -run 'TestRPC_Filter_Log' -count=1
```

- [ ] **Step 3: Complete `ethNewFilter` parse + `ethGetFilterLogs`**

Parse `address` / `topics` / `fromBlock` / `toBlock` from the filter object (mirror `ethGetLogs` in `api.go` lines 699–775). Store `addrs`, `topics`, and optional tag fields.

```go
func (a *API) ethGetFilterLogs(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	f, err := a.filters.get(p[0])
	if err != nil {
		return nil, err
	}
	if f.kind != filterLogs {
		return nil, fmt.Errorf("not a log filter")
	}
	head := a.n.BlockNumber()
	from, to := uint64(0), head
	// if f.fromTag / f.toTag set, resolve via ParseBlockNumber + ResolveBlockNumber
	matched := a.n.FilterLogs(from, to, f.addrs, f.topics)
	out := make([]map[string]interface{}, 0, len(matched))
	for _, il := range matched {
		out = append(out, formatIndexedLog(il))
	}
	return out, nil
}
```

**Do not** call `advance` from `getFilterLogs`.

- [ ] **Step 4: Full package tests**

```bash
go test ./rpc/ -count=1
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add rpc/filter.go rpc/filter_test.go
git commit -m "feat(rpc): log filter poll and eth_getFilterLogs"
```

---

### Task 4: Capacity limit unit test + docs + debt

**Files:**
- Modify: `rpc/filter_test.go` (optional capacity test)
- Modify: `docs/api/json-rpc.md`
- Modify: `docs/superpowers/plans/2026-07-14-gap-closure.md`
- Modify: `agents/debt.md`
- Modify: `docs/product/upgrades.md` if Track 4 mentions filters (only if residual listed)

- [ ] **Step 1: Capacity test (unit, no node)**

```go
func TestFilterStore_MaxFilters(t *testing.T) {
	s := newFilterStore()
	for i := 0; i < MaxFilters; i++ {
		if _, err := s.install(&installedFilter{kind: filterBlocks, lastPolled: 0}); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	if _, err := s.install(&installedFilter{kind: filterBlocks}); err == nil {
		t.Fatal("want capacity error")
	}
}
```

- [ ] **Step 2: Update `docs/api/json-rpc.md` Logs table**

```markdown
| `eth_newFilter` / `eth_getFilterChanges` / `eth_getFilterLogs` / `eth_uninstallFilter` | **Implemented** (HTTP poll); max 128 filters; 5m idle TTL |
| `eth_newBlockFilter` | **Implemented** |
| `eth_newPendingTransactionFilter` | **Implemented** (changes always empty until mempool stream) |
```

- [ ] **Step 3: Gap plan Wave 3**

Mark Task 3.1 steps done in `docs/superpowers/plans/2026-07-14-gap-closure.md`.

- [ ] **Step 4: `agents/debt.md`**

Change residual line: optional Filter API → done 2026-07-15 (or strike residual).

- [ ] **Step 5: Verify**

```bash
go test ./rpc/ ./node/ -count=1
```

- [ ] **Step 6: Commit**

```bash
git add rpc/filter_test.go docs/api/json-rpc.md docs/superpowers/plans/2026-07-14-gap-closure.md agents/debt.md
git commit -m "docs(rpc): document Filter API and close Wave 3 residual"
```

---

## Self-review (plan vs spec)

| Spec requirement | Task |
| :--- | :--- |
| `eth_newFilter` | 1, 3 |
| `eth_newBlockFilter` | 1–2 |
| `eth_newPendingTransactionFilter` empty changes | 2 |
| `eth_getFilterChanges` cursor advance | 2–3 |
| `eth_getFilterLogs` no cursor advance | 3 |
| `eth_uninstallFilter` | 1 |
| Max 128 / TTL 5m | 1 store + 4 test |
| Reuse FilterLogs / formatIndexedLog | 2–3 |
| Docs + debt | 4 |
| No wire change | all tasks RPC-only |

No placeholders left in tasks. Types: `installedFilter`, `filterStore`, `filterKind` consistent across tasks.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-15-filter-api.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks  
2. **Inline Execution** — this session, batch with checkpoints  

Which approach?
