# P1f Verified Source / ABI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist and display contract ABI (+ optional source) via `dewindex` SQLite and explorer Contract tab, with “ABI registered” semantics (no solc bytecode match).

**Architecture:** Extend indexer store with `contracts` table; HTTP `GET`/`POST /v1/contract/{addr}` with optional verify token, body limit 512 KiB, per-IP rate limit. Explorer fetches/registers when `PUBLIC_INDEXER_URL` is set. RPC/wire freeze unchanged.

**Tech Stack:** Go (`indexer/`, `cmd/dewindex`), React explorer (`explorer/src`), SQLite (`modernc.org/sqlite`), existing compose/nginx (POST already allowed on `/indexer/`).

**Spec:** [2026-07-15-p1f-verified-source-design.md](../specs/2026-07-15-p1f-verified-source-design.md)

---

## File map

| File | Role |
| :--- | :--- |
| `indexer/store.go` | Migrate `contracts` table; Get/Upsert contract |
| `indexer/contract.go` | Types, validation, rate limiter helpers (or colocate in server) |
| `indexer/server.go` | Routes GET/POST, CORS POST, token check, body limit |
| `indexer/ingest.go` | `Config.VerifyToken` field |
| `cmd/dewindex/main.go` | Flag/env `INDEXER_VERIFY_TOKEN` |
| `indexer/contract_test.go` or extend `indexer_test.go` | Store + HTTP tests |
| `explorer/src/lib/indexer.ts` | Client GET/POST + types |
| `explorer/src/hooks/use-chain.ts` | React Query hooks for contract meta |
| `explorer/src/routes/address-page.tsx` | Contract tab UI + register form |
| `explorer/src/lib/abi.ts` | Optional: resolve selector from full ABI (stretch) |
| `docs/product/indexer.md` | API docs |
| `docs/product/block-explorer.md` | P1f surface |
| `docs/product/upgrades.md` | Mark P1f shipped |
| `agents/debt.md` | Close P1f residual |

---

### Task 1: Store schema + Get/Upsert

**Files:**
- Modify: `indexer/store.go`
- Create: `indexer/contract_test.go` (package `indexer` for unit tests)

- [ ] **Step 1: Failing unit test**

```go
// indexer/contract_test.go
package indexer

import (
	"path/filepath"
	"testing"
)

func TestStore_ContractUpsert(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	addr := "0xAbcDef0123456789AbcDef0123456789AbcDef01"
	abi := `[{"type":"function","name":"foo","inputs":[],"outputs":[]}]`
	if err := s.UpsertContract(ContractRecord{
		Address:  addr,
		Name:     "Foo",
		ABIJSON:  abi,
		Source:   "contract Foo {}",
		Compiler: "solc 0.8.24",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetContract(addr)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Name != "Foo" || got.ABIJSON != abi {
		t.Fatalf("%+v", got)
	}
	// lowercase key
	if got.Address != normalizeAddr(addr) {
		t.Fatalf("addr = %s", got.Address)
	}
	// upsert
	if err := s.UpsertContract(ContractRecord{
		Address: addr,
		ABIJSON: `[]`,
		Name:    "Bar",
	}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.GetContract(addr)
	if got.Name != "Bar" || got.ABIJSON != `[]` {
		t.Fatalf("upsert: %+v", got)
	}
}
```

- [ ] **Step 2: Run — expect fail**

```bash
go test ./indexer/ -run TestStore_ContractUpsert -count=1
```

- [ ] **Step 3: Implement**

In `migrate()` append:

```sql
CREATE TABLE IF NOT EXISTS contracts (
  address    TEXT PRIMARY KEY,
  name       TEXT,
  abi_json   TEXT NOT NULL,
  source     TEXT,
  compiler   TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
```

```go
type ContractRecord struct {
	Address   string
	Name      string
	ABIJSON   string
	Source    string
	Compiler  string
	CreatedAt int64
	UpdatedAt int64
}

func (s *Store) GetContract(addr string) (ContractRecord, bool, error) {
	a := normalizeAddr(addr)
	var r ContractRecord
	err := s.db.QueryRow(`
SELECT address, COALESCE(name,''), abi_json, COALESCE(source,''), COALESCE(compiler,''), created_at, updated_at
FROM contracts WHERE address = ?`, a).Scan(
		&r.Address, &r.Name, &r.ABIJSON, &r.Source, &r.Compiler, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return ContractRecord{}, false, nil
	}
	if err != nil {
		return ContractRecord{}, false, err
	}
	return r, true, nil
}

func (s *Store) UpsertContract(r ContractRecord) error {
	a := normalizeAddr(r.Address)
	now := time.Now().Unix()
	_, err := s.db.Exec(`
INSERT INTO contracts(address, name, abi_json, source, compiler, created_at, updated_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(address) DO UPDATE SET
  name=excluded.name,
  abi_json=excluded.abi_json,
  source=excluded.source,
  compiler=excluded.compiler,
  updated_at=excluded.updated_at`,
		a, r.Name, r.ABIJSON, r.Source, r.Compiler, now, now)
	// On conflict, preserve created_at — better SQL:
	// INSERT ... ON CONFLICT DO UPDATE SET ..., updated_at=excluded.updated_at
	// and created_at only on insert (default above). For preserve created_at use:
	// created_at = COALESCE((SELECT created_at FROM contracts WHERE address=excluded.address), excluded.created_at)
	// Simpler: read existing first, or use two-step. Prefer:
	return err
}
```

Prefer upsert that keeps `created_at`:

```sql
INSERT INTO contracts(address, name, abi_json, source, compiler, created_at, updated_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(address) DO UPDATE SET
  name=excluded.name,
  abi_json=excluded.abi_json,
  source=excluded.source,
  compiler=excluded.compiler,
  updated_at=excluded.updated_at
```

`created_at` stays from first insert (SQLite does not replace it if not listed in DO UPDATE).

- [ ] **Step 4: Test pass**

```bash
go test ./indexer/ -run TestStore_ContractUpsert -count=1
```

- [ ] **Step 5: Commit**

```bash
git add indexer/store.go indexer/contract_test.go
git commit -m "feat(indexer): contracts table for P1f ABI registry"
```

---

### Task 2: HTTP GET/POST + auth + limits

**Files:**
- Modify: `indexer/server.go`, `indexer/ingest.go` (`Config`), `cmd/dewindex/main.go`
- Modify: `indexer/contract_test.go` or `indexer_test.go`

- [ ] **Step 1: HTTP tests**

```go
func TestHTTP_ContractRegisterAndGet(t *testing.T) {
	// OpenStore temp, New Indexer with empty rpc optional — or only Store+Server
	// Easiest: OpenStore, Indexer{store}, NewServer, Start on :0
	// POST /v1/contract/0xf39F... with body {"abi":[...],"name":"T"}
	// GET same → 200 status registered
	// GET unknown → 404
	// POST invalid abi object {} → 400
}
```

Wire `Server` to hold `verifyToken string` and rate limiter from `ix.cfg`.

- [ ] **Step 2: Implement handlers**

Constants:

```go
const (
	maxContractBody = 512 * 1024
	contractPOSTMax = 10
	contractPOSTWin = 10 * time.Minute
)
```

```go
// Config
VerifyToken string // empty = open

// Server fields
verifyToken string
rateMu sync.Mutex
rateByIP map[string][]time.Time
```

`NewServer`: set token from `ix.cfg.VerifyToken`; init map; register:

```go
mux.HandleFunc("/v1/contract/", s.handleContract)
```

CORS:

```go
w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Dew-Verify-Token")
```

`handleContract`:

1. Path: `/v1/contract/{addr}` only (no extra segments).  
2. Validate address like address handler.  
3. GET → GetContract → 404 or JSON with `status:"registered"`, parse abi_json to `json.RawMessage` or `json.Unmarshal` into `[]interface{}` for `abi` field.  
4. POST → check token if set; `LimitReader` body; decode JSON; require `abi` array (re-marshal to string for store); name/source/compiler length checks; rate limit by `X-Forwarded-For` first IP or `RemoteAddr`; Upsert; return 200.  

JSON response shape (spec):

```json
{
  "address": "0x…",
  "name": "…",
  "abi": [ ],
  "source": "…",
  "compiler": "…",
  "status": "registered",
  "createdAt": 0,
  "updatedAt": 0
}
```

Token check:

```go
func (s *Server) authorizeVerify(r *http.Request) bool {
	if s.verifyToken == "" {
		return true
	}
	if h := r.Header.Get("X-Dew-Verify-Token"); h == s.verifyToken {
		return true
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == s.verifyToken
	}
	return false
}
```

`cmd/dewindex`:

```go
verifyToken := flag.String("verify-token", envOr("INDEXER_VERIFY_TOKEN", ""), "optional token for POST /v1/contract")
// cfg.VerifyToken = *verifyToken
```

- [ ] **Step 3: Tests pass**

```bash
go test ./indexer/ -count=1
```

- [ ] **Step 4: Commit**

```bash
git add indexer/server.go indexer/ingest.go cmd/dewindex/main.go indexer/*_test.go
git commit -m "feat(indexer): GET/POST /v1/contract for ABI registration"
```

---

### Task 3: Explorer client + Contract tab UI

**Files:**
- Modify: `explorer/src/lib/indexer.ts`
- Modify: `explorer/src/hooks/use-chain.ts`
- Modify: `explorer/src/routes/address-page.tsx`
- Optional: `explorer/src/lib/query-keys.ts`

- [ ] **Step 1: Client API**

```ts
export type ContractMeta = {
  address: string;
  name?: string;
  abi: unknown[];
  source?: string;
  compiler?: string;
  status: "registered";
  createdAt: number;
  updatedAt: number;
};

export async function fetchContractMeta(addr: string): Promise<ContractMeta | null> {
  const b = base();
  if (!b) return null;
  const res = await fetch(`${b}/v1/contract/${encodeURIComponent(addr)}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`indexer HTTP ${res.status}`);
  return (await res.json()) as ContractMeta;
}

export async function registerContract(
  addr: string,
  body: { name?: string; abi: unknown[]; source?: string; compiler?: string },
): Promise<ContractMeta> {
  const b = base();
  if (!b) throw new Error("indexer not configured");
  const res = await fetch(`${b}/v1/contract/${encodeURIComponent(addr)}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const t = await res.text();
    throw new Error(t || `HTTP ${res.status}`);
  }
  return (await res.json()) as ContractMeta;
}
```

- [ ] **Step 2: Hooks**

```ts
export function useContractMeta(addr: string, enabled: boolean) {
  return useQuery({
    queryKey: ["contract-meta", addr.toLowerCase()],
    queryFn: () => fetchContractMeta(addr),
    enabled: enabled && indexerEnabled() && isHexAddress(addr),
    staleTime: 30_000,
  });
}
```

- [ ] **Step 3: Address Contract tab**

When `isContract`:

- Load `useContractMeta(addr, isContract)`.  
- Badge in header or tab: if meta → “ABI registered”.  
- Content:
  - If meta: show name, compiler, collapsible ABI (`JSON.stringify(abi, null, 2)`), source pre, bytecode pre.  
  - If !meta && hasIndexer: form with controlled textareas; parse ABI JSON on submit; `registerContract` + `queryClient.invalidateQueries`.  
  - Loading / error states soft.

Keep existing bytecode block.

- [ ] **Step 4: Build**

```bash
pnpm --dir explorer build
go test ./indexer/ -count=1
```

- [ ] **Step 5: Commit**

```bash
git add explorer/src/
git commit -m "feat(explorer): Contract tab ABI registered + register form (P1f)"
```

---

### Task 4: Docs + debt + optional tx decode stretch

**Files:**
- Modify: `docs/product/indexer.md`, `docs/product/block-explorer.md`, `docs/product/upgrades.md`, `agents/debt.md`
- Optional stretch: `explorer/src/lib/abi.ts` + `tx-page.tsx` method name from registered ABI (skip if timeboxed — note in debt as optional follow-up)

- [ ] **Step 1: indexer.md**

Add section **Contract ABI registry (P1f)** with GET/POST, env `INDEXER_VERIFY_TOKEN`, rate limits, badge wording.

- [ ] **Step 2: block-explorer.md**

Note Contract tab ABI registered + form when indexer configured.

- [ ] **Step 3: upgrades.md**

P1f row → **Shipped** (date). Deferred list checkbox done.

- [ ] **Step 4: debt.md**

Remove P1f from residuals; note done 2026-07-15.

- [ ] **Step 5: Verify**

```bash
go test ./indexer/ ./rpc/ -count=1
pnpm --dir explorer build
```

- [ ] **Step 6: Commit**

```bash
git add docs/ agents/debt.md
git commit -m "docs(product): ship P1f verified source/ABI registry"
```

---

## Self-review (plan vs spec)

| Spec item | Task |
| :--- | :--- |
| `contracts` table | 1 |
| GET/POST API + status registered | 2 |
| Optional token | 2 |
| Body 512 KiB + rate limit | 2 |
| CORS POST | 2 |
| Explorer badge + form | 3 |
| Docs + debt | 4 |
| Tx ABI decode | optional stretch Task 4 |
| No wire change | all |

Nginx: no change required (POST already allowed).

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-15-p1f-verified-source.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — fresh subagent per task  
2. **Inline Execution** — this session  

Which approach?
