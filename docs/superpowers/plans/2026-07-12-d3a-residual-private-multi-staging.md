# D3a residual + private multi-host staging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the multiproc empty-block stall residual (pace + light bulk backpressure) and document private Compose `multi` staging so Path A ops can follow later.

**Architecture:** Raise default `MinBlockInterval` to 1s and expose `--bft.min-block-interval`. Make non-consensus P2P sends non-blocking when the peer outbound queue is full so vote/proposal frames are not starved. Gate with a multiproc soak test (≥150 heights) and refresh private-testnet / deploy docs.

**Tech Stack:** Go monorepo (`consensus/`, `p2p/`, `node/`, `cmd/dew`, `devnet/`), Docker Compose soak profile `multi`, markdown under `docs/` and `deploy/`.

**Spec:** [docs/superpowers/specs/2026-07-12-d3a-residual-private-multi-staging-design.md](../specs/2026-07-12-d3a-residual-private-multi-staging-design.md)

---

## File map

| File | Responsibility |
| :--- | :--- |
| `consensus/runner.go` | Default interval **1s**; comment accuracy |
| `consensus/runner_test.go` | Assert default pacing constant / optional wait behavior |
| `p2p/peer.go` | Blocking vs non-blocking `Send` |
| `p2p/host.go` | `Broadcast` chooses blocking by message type |
| `p2p/protocol.go` | Optional helper `IsConsensusMsg(typ uint8) bool` |
| `p2p/peer_test.go` or `p2p/backpressure_test.go` | Queue-full bulk drop + consensus wait |
| `node/stack.go` | `StackConfig.MinBlockInterval` → `Runner` |
| `cmd/dew/main.go` | Flag `--bft.min-block-interval` |
| `devnet/multiprocess_bft_test.go` | Long empty multiproc soak |
| `agents/debt.md`, `docs/development/private-testnet.md`, `docs/development/d3-scale.md`, `docs/development/phases.md`, `deploy/README.md` | Close residual + honest multi runbook |

---

### Task 1: Raise default MinBlockInterval to 1s

**Files:**
- Modify: `consensus/runner.go`
- Modify: `consensus/runner_test.go`

- [ ] **Step 1: Write a failing assertion on the default constant**

In `consensus/runner_test.go`, add:

```go
func TestDefaultMinBlockInterval(t *testing.T) {
	if defaultMinBlockInterval != time.Second {
		t.Fatalf("defaultMinBlockInterval=%v want 1s", defaultMinBlockInterval)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./consensus/ -run TestDefaultMinBlockInterval -count=1`

Expected: FAIL (constant is still `200 * time.Millisecond`).

- [ ] **Step 3: Update default and comment**

In `consensus/runner.go`:

```go
const defaultMinBlockInterval = time.Second
```

Update the struct field comment to:

```go
// MinBlockInterval paces StartRound after commit (curbs empty-block storms).
// Zero means defaultMinBlockInterval (1s); negative disables pacing.
MinBlockInterval time.Duration
```

Keep existing `if interval == 0 { interval = defaultMinBlockInterval }` logic.

- [ ] **Step 4: Run tests**

Run: `go test ./consensus/ -count=1`

Expected: PASS (existing tests already set `MinBlockInterval: -1`).

- [ ] **Step 5: Commit**

```bash
git add consensus/runner.go consensus/runner_test.go
git commit -m "fix(consensus): default MinBlockInterval 1s for multiproc pace"
```

---

### Task 2: Wire CLI + StackConfig for min block interval

**Files:**
- Modify: `node/stack.go`
- Modify: `cmd/dew/main.go`

- [ ] **Step 1: Add field on StackConfig**

In `node/stack.go` on `StackConfig`:

```go
// MinBlockInterval paces BFT StartRound after commit (0 = consensus default 1s; negative disables).
MinBlockInterval time.Duration
```

When constructing the runner (validator branch only):

```go
runner := &consensus.Runner{
	Engine:           engine,
	MinBlockInterval: cfg.MinBlockInterval,
	OnCommit: func(ev consensus.CommitEvent) error {
		// existing body unchanged
	},
}
```

- [ ] **Step 2: Add CLI flag**

In `cmd/dew/main.go` `cmdRun`, after other flags:

```go
minBlockInterval := fs.String("bft.min-block-interval", "", "min time between committed heights (Go duration; empty=default 1s; negative disables e.g. -1ns)")
```

After `fs.Parse`, parse if non-empty:

```go
var minBlock time.Duration
if s := strings.TrimSpace(*minBlockInterval); s != "" {
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("bft.min-block-interval: %w", err)
	}
	minBlock = d
}
```

Pass into both validator and full-node `StackConfig` blocks:

```go
MinBlockInterval: minBlock,
```

(`0` when flag empty → Runner uses default 1s.)

- [ ] **Step 3: Manual smoke of flag parse**

Run: `go build -o bin/dew ./cmd/dew && ./bin/dew run -h 2>&1 | grep min-block`  
(or start with invalid duration and expect error)

Expected: flag listed; invalid duration returns non-zero.

- [ ] **Step 4: Commit**

```bash
git add node/stack.go cmd/dew/main.go
git commit -m "feat(dew): --bft.min-block-interval for validator pace"
```

---

### Task 3: Consensus vs bulk Send backpressure

**Files:**
- Modify: `p2p/protocol.go` (helper)
- Modify: `p2p/peer.go`
- Modify: `p2p/host.go`
- Create or modify: `p2p/backpressure_test.go`

- [ ] **Step 1: Add message class helper**

In `p2p/protocol.go`:

```go
// IsConsensusMsg reports Dew-BFT wire types that must not be dropped under load.
func IsConsensusMsg(typ uint8) bool {
	return typ == MsgProposal || typ == MsgPrevote || typ == MsgPrecommit
}
```

- [ ] **Step 2: Write failing backpressure tests**

Create `p2p/backpressure_test.go`:

```go
package p2p

import (
	"testing"
	"time"
)

func TestPeer_BulkSendDropsWhenQueueFull(t *testing.T) {
	// Peer with tiny outCh and no writeLoop consumer.
	p := &Peer{
		outCh:   make(chan outMsg, 1),
		closeCh: make(chan struct{}),
		host:    &Host{cfg: Config{WriteTimeout: 2 * time.Second}},
	}
	// Fill buffer.
	if err := p.Send(MsgInventory, []byte("a")); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := p.Send(MsgInventory, []byte("b")) // bulk — must not wait WriteTimeout
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("bulk drop should return nil, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("bulk send blocked %v; want non-blocking drop", elapsed)
	}
}

func TestPeer_ConsensusSendWaitsWhenQueueFull(t *testing.T) {
	p := &Peer{
		outCh:   make(chan outMsg, 1),
		closeCh: make(chan struct{}),
		host:    &Host{cfg: Config{WriteTimeout: 50 * time.Millisecond}},
	}
	if err := p.Send(MsgProposal, []byte("a")); err != nil {
		t.Fatal(err)
	}
	// Unblock after short delay so we observe wait, not permanent hang.
	go func() {
		time.Sleep(20 * time.Millisecond)
		select {
		case <-p.outCh:
		default:
		}
	}()
	start := time.Now()
	err := p.Send(MsgProposal, []byte("b"))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("consensus send: %v", err)
	}
	if elapsed < 10*time.Millisecond {
		t.Fatalf("consensus send returned too fast (%v); expected short wait", elapsed)
	}
}
```

Adjust if `Send` signature changes — tests define intended behavior.

- [ ] **Step 3: Run tests to verify fail**

Run: `go test ./p2p/ -run 'BulkSend|ConsensusSend' -count=1`

Expected: FAIL (current `Send` always blocks for bulk).

- [ ] **Step 4: Implement Send with blocking policy**

In `p2p/peer.go`, replace `Send` body selection with:

```go
// Send queues a framed message. Consensus types wait up to WriteTimeout;
// bulk types drop immediately if the outbound queue is full.
func (p *Peer) Send(typ uint8, payload []byte) error {
	if p.closed.Load() {
		return fmt.Errorf("p2p: peer closed")
	}
	msg := outMsg{typ: typ, payload: append([]byte(nil), payload...)}
	if IsConsensusMsg(typ) {
		return p.enqueueBlocking(msg)
	}
	return p.enqueueDrop(msg)
}

func (p *Peer) enqueueDrop(msg outMsg) error {
	select {
	case p.outCh <- msg:
		return nil
	case <-p.closeCh:
		return fmt.Errorf("p2p: peer closed")
	default:
		return nil // drop bulk under pressure
	}
}

func (p *Peer) enqueueBlocking(msg outMsg) error {
	timeout := p.host.cfg.WriteTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case p.outCh <- msg:
		return nil
	case <-p.closeCh:
		return fmt.Errorf("p2p: peer closed")
	case <-timer.C:
		return fmt.Errorf("p2p: outbound queue timeout")
	}
}
```

Ensure `p.host` is non-nil in tests (set as above). Production `newPeer` already sets `host`.

- [ ] **Step 5: Run p2p tests**

Run: `go test ./p2p/ -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add p2p/protocol.go p2p/peer.go p2p/host.go p2p/backpressure_test.go
git commit -m "fix(p2p): drop bulk outbound when queue full; keep consensus blocking"
```

---

### Task 4: Multi-process long empty soak test

**Files:**
- Modify: `devnet/multiprocess_bft_test.go`
- Optionally: `devnet/multiprocess.go` if `MinBlockInterval` must be set on stacks for faster CI

- [ ] **Step 1: Prefer test-friendly interval for long soak**

Long soak at 1s × 150 heights ≈ 2.5+ minutes plus BFT overhead. For CI reliability:

- Production default stays **1s**.
- Long-run test may set `StackConfig.MinBlockInterval` to **50ms** or **100ms** only if that still exercises the residual path; **prefer production default 1s** with timeout **5 minutes** if machine allows.

If default 1s is too slow for routine `go test ./devnet/`, gate the test:

```go
if os.Getenv("DEW_HEAVY_INTEGRATION") == "" {
	t.Skip("set DEW_HEAVY_INTEGRATION=1 for multiproc empty soak ≥150 heights")
}
```

And document that heavy mode uses default pacing (or explicit 200ms) — **must not use `-1`** (disables residual fix under test).

Recommended for heavy test: `MinBlockInterval: 200 * time.Millisecond` via extending `MultiProcessConfig` + `StartMultiProcessBFT` to pass interval into each validator stack — still validates pace + backpressure without 3+ minute waits. Acceptance “≥150 heights” remains.

Extend `MultiProcessConfig`:

```go
// MinBlockInterval applied to validator runners (0 = consensus default).
MinBlockInterval time.Duration
```

Pass through in `StartStack` configs for validators.

- [ ] **Step 2: Write TestMultiProcessBFT_LongEmpty**

```go
func TestMultiProcessBFT_LongEmpty(t *testing.T) {
	if os.Getenv("DEW_HEAVY_INTEGRATION") == "" {
		t.Skip("set DEW_HEAVY_INTEGRATION=1 for multiproc empty soak")
	}
	const targetHeight uint64 = 150
	netw, err := StartMultiProcessBFT(MultiProcessConfig{
		HTTPAddr:         "127.0.0.1:0",
		MinBlockInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = netw.Stop() })

	// Optional: keep full node catch-up ticker like ERC20 test.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				catchUpFullNode(netw)
			}
		}
	}()
	defer close(done)

	tipHash, tipNum := waitUniformTip(t, netw, targetHeight, 4*time.Minute)
	if tipNum < targetHeight {
		t.Fatalf("height=%d want >= %d", tipNum, targetHeight)
	}
	_ = tipHash
}
```

Reuse existing `waitUniformTip` / `catchUpFullNode` helpers already in the file.

- [ ] **Step 3: Run light multiproc (always)**

Run: `go test ./devnet/ -run MultiProcessBFT_SharedChain -count=1`

Expected: PASS under new default when stacks use 0 → 1s (SharedChain only needs height ≥3 — still fine within 120s).

- [ ] **Step 4: Run heavy long empty**

Run: `DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run MultiProcessBFT_LongEmpty -count=1 -timeout 5m`

Expected: PASS, height ≥150, uniform tip.

If FAIL with stall, re-check bulk drop paths and whether `Broadcast` / rebroadcast in `handlers.go` still use `Send` (they should inherit policy).

- [ ] **Step 5: Commit**

```bash
git add devnet/multiprocess.go devnet/multiprocess_bft_test.go
git commit -m "test(devnet): multiproc empty soak ≥150 heights (heavy)"
```

---

### Task 5: Docs, debt, deploy README

**Files:**
- Modify: `agents/debt.md`
- Modify: `docs/development/private-testnet.md`
- Modify: `docs/development/d3-scale.md` (status line if needed)
- Modify: `docs/development/phases.md` (D3 notes if residual mentioned)
- Modify: `deploy/README.md`
- Optional: `deploy/node/docker-compose.soak.yml` explicit flag

- [ ] **Step 1: Close residual in debt**

In `agents/debt.md`, mark:

```markdown
- [x] **D3a residual:** multiproc empty-block pace + bulk outbound drop; soak `DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run MultiProcessBFT_LongEmpty`
```

- [ ] **Step 2: Fix deploy/README.md multi section**

Replace stale “auto-mine / D3a not wired” with:

- 3 validators (`--validator`) + `node-rpc` (`--no-auto-mine`)
- Shared canonical chain (D3a)
- RPC smoke: `node scripts/smoke-rpc.mjs http://127.0.0.1:8548`
- ERC-20: `node scripts/devnet-erc20.mjs http://127.0.0.1:8548`
- Ports table already correct; add `node-rpc` `:8548`

- [ ] **Step 3: private-testnet.md**

Add short subsection:

```markdown
### Multi-process BFT soak (Compose `multi`)

1. `docker compose -f deploy/node/docker-compose.soak.yml --profile multi up --build`
2. Smoke RPC on `:8548`; confirm `eth_blockNumber` advances.
3. ERC-20: `node scripts/devnet-erc20.mjs http://127.0.0.1:8548`
4. Kill one validator container; within ~2 min peers recover (D3b) and tip continues.
5. Optional pace: `--bft.min-block-interval 1s` (default).
```

- [ ] **Step 4: phases / d3-scale one-liners**

Note residual closed (July 2026) next to D3a/D3b done lines.

- [ ] **Step 5: Commit**

```bash
git add agents/debt.md docs/development/private-testnet.md docs/development/d3-scale.md docs/development/phases.md deploy/README.md deploy/node/docker-compose.soak.yml
git commit -m "docs: close D3a residual and fix multi-process private runbook"
```

---

### Task 6: Verification gate

- [ ] **Step 1: Full package tests**

```bash
go test ./consensus/ ./p2p/ ./node/ ./devnet/ -count=1 -timeout 3m
```

Expected: PASS (LongEmpty skipped without env).

- [ ] **Step 2: Heavy soak**

```bash
DEW_HEAVY_INTEGRATION=1 go test ./devnet/ -run 'MultiProcessBFT_' -count=1 -timeout 10m
```

Expected: SharedChain + LongEmpty (+ ERC20 if included) PASS.

- [ ] **Step 3: Build CLI**

```bash
go build -o bin/dew ./cmd/dew
```

Expected: success.

---

## Spec coverage checklist

| Spec requirement | Task |
| :--- | :---: |
| Default MinBlockInterval 1s | 1 |
| CLI `--bft.min-block-interval` | 2 |
| Bulk drop / consensus blocking | 3 |
| Multiproc ≥150 heights test | 4 |
| Docs / debt / deploy README | 5 |
| Acceptance verification | 6 |

## Out of scope (do not implement in this plan)

- Path A public keys/bootnodes (D3d)
- Stale consensus coalesce
- D3c staking
- Raising MaxTxsPerBlock / fee auction
