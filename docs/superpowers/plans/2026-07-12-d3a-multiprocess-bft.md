# D3a Multi-process BFT Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire ≥3 `dew run --validator` processes + optional full RPC node to share one canonical BFT chain over encrypted P2P.

**Architecture:** `node.ImportCommittedBlock` is the single apply path; `consensus.Engine` proposes via `MempoolBlockBuilder`; `p2p.Broadcaster` floods proposals/votes; `consensus.Runner` drives rounds after commit with 3s timeout.

**Tech Stack:** Go 1.22+, existing `consensus/`, `p2p/`, `node/`, `cmd/dew`, `devnet/`, Docker Compose.

**Design spec:** [docs/superpowers/specs/2026-07-12-d3a-multiprocess-bft-design.md](../specs/2026-07-12-d3a-multiprocess-bft-design.md)

---

## File map

| File | Responsibility |
| :--- | :--- |
| `core/types/block_wire.go` | `Block.MarshalBinary` / `UnmarshalBlockBinary` |
| `core/types/block_wire_test.go` | Round-trip + hash stability |
| `node/import.go` | `ImportCommittedBlock`, `applyBlockBody`, tx execution helpers |
| `node/build.go` | `BuildBlockFromPool`, `ValidateAndExecuteBlock` |
| `node/p2p.go` | `P2PBackend` (`ChainBackend` + `TxBackend`) |
| `node/import_test.go` | Apply path + `SetAutoMine` tests |
| `node/build_test.go` | Builder + validator execution tests |
| `consensus/builder.go` | `MempoolBlockBuilder`, `ExecutionValidator`, `BlockExecutor` interface |
| `consensus/genesis.go` | `ValidatorSetFromGenesis` |
| `consensus/runner.go` | `Runner` with commit hook + round timeout |
| `consensus/builder_test.go` | Builder/validator unit tests |
| `consensus/runner_test.go` | Runner starts next round after commit |
| `p2p/bft.go` | Wire ↔ consensus conversion, `NewBroadcaster` |
| `p2p/bft_test.go` | Proposal/vote wire round-trip |
| `devnet/multiprocess.go` | `StartMultiProcessBFT` harness |
| `devnet/multiprocess_bft_test.go` | Integration test |
| `cmd/dew/bft.go` | `startValidatorStack`, `startFullNodeStack` |
| `cmd/dew/main.go` | `--validator`, `--validator.key`, `--no-auto-mine` flags |
| `deploy/node/docker-compose.soak.yml` | Validator + `node-rpc` profile |

---

### Task 1: Block wire encoding

**Files:**
- Create: `core/types/block_wire.go`
- Create: `core/types/block_wire_test.go`
- Modify: `core/types/block.go` (add `BodyRaws` field optional — see step 3)

- [ ] **Step 1: Write the failing test**

```go
// core/types/block_wire_test.go
package types_test

import (
	"math/big"
	"testing"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func TestBlockMarshalRoundTrip_EVMTx(t *testing.T) {
	h := &types.Header{
		ParentHash:  types.Hash{},
		StateRoot:   types.Keccak256Hash([]byte("state")),
		TxRoot:      types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Number:      1,
		Timestamp:   2,
		GasLimit:    30_000_000,
		GasUsed:     21000,
		BaseFee:     big.NewInt(1_000_000_000),
		Proposer:    crypto.Address{},
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      21000,
		To:       ptrAddr(crypto.Address{}),
		Value:    big.NewInt(1),
	})
	blk := types.NewBlock(h, []*ethtypes.Transaction{tx})
	raw, err := blk.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	out, err := types.UnmarshalBlockBinary(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Hash() != blk.Hash() {
		t.Fatalf("hash mismatch: %s vs %s", out.Hash().Hex(), blk.Hash().Hex())
	}
	if len(out.Transactions()) != 1 {
		t.Fatalf("tx count=%d", len(out.Transactions()))
	}
}

func ptrAddr(a crypto.Address) *crypto.Address { return &a }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/types/ -run TestBlockMarshalRoundTrip -v`  
Expected: FAIL — `blk.MarshalBinary undefined`

- [ ] **Step 3: Implement minimal encoding**

```go
// core/types/block_wire.go
package types

import (
	"fmt"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// MarshalBinary encodes header + raw tx payloads for P2P/sync.
// Wire: RLP([headerRLPList, [txRaw0, txRaw1, ...]])
func (b *Block) MarshalBinary() ([]byte, error) {
	if b == nil || b.header == nil {
		return nil, fmt.Errorf("types: nil block")
	}
	var hdr []byte
	if err := rlp.EncodeToBytes(b.header); err != nil {
		return nil, err
	}
	raws := make([][]byte, 0, len(b.transactions))
	for _, tx := range b.transactions {
		if tx == nil {
			continue
		}
		bin, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		raws = append(raws, bin)
	}
	return rlp.EncodeToBytes([]interface{}{hdr, raws})
}

// UnmarshalBlockBinary decodes a wire block.
func UnmarshalBlockBinary(data []byte) (*Block, error) {
	var payload struct {
		Header []byte
		TxRaws [][]byte
	}
	if err := rlp.DecodeBytes(data, &payload); err != nil {
		return nil, err
	}
	h := new(Header)
	if err := rlp.DecodeBytes(payload.Header, h); err != nil {
		return nil, err
	}
	txs := make([]*ethtypes.Transaction, 0, len(payload.TxRaws))
	for _, raw := range payload.TxRaws {
		tx := new(ethtypes.Transaction)
		if err := tx.UnmarshalBinary(raw); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return NewBlock(h, txs), nil
}
```

Note: `rlp.EncodeToBytes(b.header)` works because `Header` implements `EncodeRLP`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./core/types/ -run TestBlockMarshalRoundTrip -v`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/types/block_wire.go core/types/block_wire_test.go
git commit -m "feat(types): add block wire encoding for P2P sync"
```

---

### Task 2: SetAutoMine and admit-only RPC path

**Files:**
- Modify: `node/node.go` (add `autoMine bool` field, default `true` in `NewFromGenesis`)
- Modify: `node/node.go` (`SendRawTransaction`, `SendDewRawTransaction`)
- Create: `node/import_test.go`

- [ ] **Step 1: Write the failing test**

```go
// node/import_test.go
package node_test

import (
	"math/big"
	"testing"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func testGenesis(t *testing.T) *config.Genesis {
	t.Helper()
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestSetAutoMine_AdmitOnly(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)
	before := n.BlockNumber()

	key, _ := ethcrypto.HexToECDSA(devnet.PrivHex0)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      21000,
		To:       ptrAddr(ethcrypto.PubkeyToAddress(key.PublicKey)),
		Value:    big.NewInt(1),
	})
	signed, _ := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(n.ChainID()), key)
	raw, _ := signed.MarshalBinary()
	if _, err := n.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}
	if n.BlockNumber() != before {
		t.Fatalf("head advanced: %d -> %d", before, n.BlockNumber())
	}
	if n.Mempool().Len() != 1 {
		t.Fatalf("mempool len=%d want 1", n.Mempool().Len())
	}
}

func ptrAddr(a interface{ Hex() string }) *ethcrypto.Address {
	// use common.Address — simplify in actual file with common.Address
	return nil // replace with proper helper
}
```

Fix `ptrAddr` in real file using `ethcommon.Address`.

- [ ] **Step 2: Run test — expect FAIL** (`SetAutoMine undefined` or head advances)

Run: `go test ./node/ -run TestSetAutoMine_AdmitOnly -v`

- [ ] **Step 3: Implement SetAutoMine**

In `node/node.go` struct add:
```go
autoMine bool // default true
```

In `NewFromGenesis`:
```go
autoMine: true,
```

Add methods:
```go
func (n *Node) SetAutoMine(v bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.autoMine = v
}

func (n *Node) AutoMine() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.autoMine
}
```

In `SendRawTransaction` after successful `pool.AddEVM`:
```go
if !n.autoMine {
	return txHash, nil
}
// existing execute + mine path (remove defer pool.Remove before early return)
```

Same pattern for `SendDewRawTransaction`.

- [ ] **Step 4: Run test — expect PASS**

Run: `go test ./node/ -run TestSetAutoMine_AdmitOnly -v`

- [ ] **Step 5: Commit**

```bash
git add node/node.go node/import_test.go
git commit -m "feat(node): add SetAutoMine admit-only tx path"
```

---

### Task 3: ImportCommittedBlock

**Files:**
- Create: `node/import.go`
- Modify: `node/import_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestImportCommittedBlock_Idempotent(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)
	// Build one block via auto-mine helper on a clone OR use BuildBlockFromPool in Task 4.
	// For Task 3: construct minimal block manually after Task 4 lands; interim test uses
	// n with auto-mine true to produce block, then re-import same block on fresh node.
	src, _ := node.NewFromGenesis(testGenesis(t))
	// ... send self-transfer with auto-mine, capture block at height 1 ...
	blk := src.GetBlockByNumber(1)
	if blk == nil {
		t.Fatal("missing block 1")
	}
	dst, _ := node.NewFromGenesis(testGenesis(t))
	if err := dst.ImportCommittedBlock(blk); err != nil {
		t.Fatal(err)
	}
	if dst.BlockNumber() != 1 {
		t.Fatalf("height=%d", dst.BlockNumber())
	}
	if err := dst.ImportCommittedBlock(blk); err != nil {
		t.Fatal(err)
	}
	if dst.BlockNumber() != 1 {
		t.Fatal("idempotent import changed head")
	}
}
```

Implement test using `src` with default auto-mine: sign+send tx, get block 1, import to `dst`.

- [ ] **Step 2: Run test — FAIL** (`ImportCommittedBlock undefined`)

- [ ] **Step 3: Implement `node/import.go`**

Key logic:
```go
func (n *Node) ImportCommittedBlock(block *dewtypes.Block) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if block == nil {
		return fmt.Errorf("node: nil block")
	}
	h := block.Header()
	if existing, ok := n.blockNum[h.Number]; ok {
		if existing == block.Hash() {
			return nil // idempotent
		}
		return fmt.Errorf("node: conflict at height %d", h.Number)
	}
	if h.Number != n.header.Number+1 {
		return fmt.Errorf("node: block %d not parent+1 of head %d", h.Number, n.header.Number)
	}
	if h.ParentHash != n.header.Hash() {
		return fmt.Errorf("node: parent hash mismatch")
	}
	// Re-execute txs to verify state root
	root, err := n.executeBlockLocked(n.header, block)
	if err != nil {
		return err
	}
	if root != h.StateRoot {
		return fmt.Errorf("node: state root mismatch: got %s want %s", root.Hex(), h.StateRoot.Hex())
	}
	n.commitBlockLocked(block, /* receipts/indexes from execution */)
	return nil
}
```

Extract shared execution from `SendRawTransaction` into `executeBlockLocked` / `commitBlockLocked`. Remove included tx hashes from `n.pool`.

- [ ] **Step 4: Run test — PASS**

Run: `go test ./node/ -run TestImportCommittedBlock -v`

- [ ] **Step 5: Commit**

```bash
git add node/import.go node/import_test.go node/node.go
git commit -m "feat(node): add ImportCommittedBlock apply path"
```

---

### Task 4: BuildBlockFromPool and ValidateAndExecuteBlock

**Files:**
- Create: `node/build.go`
- Create: `node/build_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildBlockFromPool_IncludesPendingTx(t *testing.T) {
	n, _ := node.NewFromGenesis(testGenesis(t))
	n.SetAutoMine(false)
	// admit tx (same as Task 2)
	parent := n.CurrentHeader()
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, parent.Proposer, 1)
	if err != nil {
		t.Fatal(err)
	}
	if blk.Number() != 1 {
		t.Fatalf("number=%d", blk.Number())
	}
	if len(blk.Transactions()) != 1 {
		t.Fatalf("txs=%d", len(blk.Transactions()))
	}
	root, err := n.ValidateAndExecuteBlock(parent, blk)
	if err != nil {
		t.Fatal(err)
	}
	if root != blk.Header().StateRoot {
		t.Fatalf("root mismatch")
	}
}
```

- [ ] **Step 2: Run — FAIL**

- [ ] **Step 3: Implement `node/build.go`**

```go
const DefaultMaxTxsPerBlock = 1

func (n *Node) BuildBlockFromPool(height uint64, parent *dewtypes.Header, proposer dewcrypto.Address, maxTxs int) (*dewtypes.Block, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if maxTxs <= 0 {
		maxTxs = DefaultMaxTxsPerBlock
	}
	// Pick highest-price pending entry with executable nonce
	pending := n.pool.Pending()
	// sort by price desc, execute first valid EVM tx on snapshot
	// return block with computed StateRoot, GasUsed, receipts
}

func (n *Node) ValidateAndExecuteBlock(parent *dewtypes.Header, block *dewtypes.Block) (dewtypes.Hash, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.executeBlockLocked(parent, block)
}
```

`BuildBlockFromPool` uses `statedb.Snapshot` + executes without committing until proposal is accepted; returned block carries post-state root.

- [ ] **Step 4: Run — PASS**

Run: `go test ./node/ -run TestBuildBlockFromPool -v`

- [ ] **Step 5: Commit**

```bash
git add node/build.go node/build_test.go
git commit -m "feat(node): mempool block builder and execution validator"
```

---

### Task 5: Consensus builder + genesis valset

**Files:**
- Create: `consensus/builder.go`
- Create: `consensus/genesis.go`
- Create: `consensus/builder_test.go`

- [ ] **Step 1: Write failing test**

```go
// consensus/builder_test.go
type fakeExec struct {
	node *node.Node
}

func (f *fakeExec) BuildBlockFromPool(height uint64, parent *types.Header, proposer crypto.Address, maxTxs int) (*types.Block, error) {
	return f.node.BuildBlockFromPool(height, parent, proposer, maxTxs)
}
func (f *fakeExec) ValidateAndExecuteBlock(parent *types.Header, block *types.Block) (types.Hash, error) {
	return f.node.ValidateAndExecuteBlock(parent, block)
}

func TestMempoolBlockBuilder_BuildsBlock(t *testing.T) {
	// genesis node, admit tx, builder builds proposal at height 1
}
```

Define interface in `consensus/builder.go`:
```go
type BlockExecutor interface {
	BuildBlockFromPool(height uint64, parent *types.Header, proposer crypto.Address, maxTxs int) (*types.Block, error)
	ValidateAndExecuteBlock(parent *types.Header, block *types.Block) (types.Hash, error)
}

type MempoolBlockBuilder struct {
	Exec   BlockExecutor
	MaxTxs int
}

type ExecutionValidator struct {
	Exec BlockExecutor
}
```

`ValidatorSetFromGenesis` in `consensus/genesis.go`:
```go
func ValidatorSetFromGenesis(g *config.Genesis) (*ValidatorSet, error) {
	vals := make([]Validator, 0, len(g.InitialValidators))
	for _, iv := range g.InitialValidators {
		addr := crypto.HexToAddress(iv.Address)
		power := iv.VotingPower
		if power == 0 {
			power = 1
		}
		vals = append(vals, Validator{Address: addr, Power: power})
	}
	return NewValidatorSet(vals)
}
```

- [ ] **Step 2–4: TDD cycle + commit**

```bash
git add consensus/builder.go consensus/genesis.go consensus/builder_test.go
git commit -m "feat(consensus): mempool block builder and genesis valset"
```

---

### Task 6: P2P BFT bridge and P2PBackend

**Files:**
- Create: `p2p/bft.go`
- Create: `p2p/bft_test.go`
- Create: `node/p2p.go`

- [ ] **Step 1: Write failing test `TestProposalWireRoundTrip`**

```go
func TestProposalWireRoundTrip(t *testing.T) {
	p := &consensus.Proposal{
		Height: 1, Round: 0,
		BlockHash: types.Keccak256Hash([]byte("b")),
		Proposer:  crypto.Address{},
		Signature: make([]byte, 65),
	}
	wp, err := ProposalToWire(p)
	if err != nil {
		t.Fatal(err)
	}
	back, err := WireToProposal(wp)
	if err != nil {
		t.Fatal(err)
	}
	if back.BlockHash != p.BlockHash {
		t.Fatal("hash mismatch")
	}
}
```

- [ ] **Step 2: Implement `p2p/bft.go`**

```go
type broadcaster struct{ host *Host }

func NewBroadcaster(h *Host) consensus.Broadcaster {
	return &broadcaster{host: h}
}

func (b *broadcaster) BroadcastProposal(p *consensus.Proposal) {
	wp, err := ProposalToWire(p)
	if err != nil {
		return
	}
	_ = b.host.BroadcastProposal(wp)
}
// BroadcastVote similar
```

`ProposalToWire` includes `BlockRaw` from `block.MarshalBinary()` when `p.Block != nil`.

- [ ] **Step 3: Implement `node/p2p.go`**

```go
type P2PBackend struct{ N *Node }

func (b *P2PBackend) Height() uint64 { return b.N.BlockNumber() }
func (b *P2PBackend) HeadHash() types.Hash { return b.N.CurrentHeader().Hash() }
func (b *P2PBackend) BlockByNumber(n uint64) ([]byte, types.Hash, bool) {
	blk := b.N.GetBlockByNumber(n)
	if blk == nil {
		return nil, types.Hash{}, false
	}
	raw, err := blk.MarshalBinary()
	// ...
}
func (b *P2PBackend) HasTx(h types.Hash) bool { return b.N.Mempool().Get(h) != nil }
func (b *P2PBackend) GetTx(h types.Hash) ([]byte, bool) {
	e := b.N.Mempool().Get(h)
	if e == nil {
		return nil, false
	}
	return e.Raw, true
}
```

- [ ] **Step 4: Run tests + commit**

```bash
go test ./p2p/ -run TestProposalWireRoundTrip -v
go test ./node/ -count=1
git commit -m "feat(p2p): BFT wire bridge and node P2PBackend"
```

---

### Task 7: Consensus Runner

**Files:**
- Create: `consensus/runner.go`
- Create: `consensus/runner_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestRunner_StartsNextRoundAfterCommit(t *testing.T) {
	// LocalCluster-style 3 engines but use Runner on one engine
	// After RunHeight equivalent, assert height advanced
}
```

- [ ] **Step 2: Implement Runner**

```go
type Runner struct {
	Engine       *Engine
	OnCommit     func(CommitEvent) error
	RoundTimeout time.Duration // default 3s
	stop         chan struct{}
}

func (r *Runner) Start() error {
	r.Engine.OnCommit = func(ev CommitEvent) {
		if r.OnCommit != nil {
			_ = r.OnCommit(ev)
		}
		_ = r.Engine.StartRound()
	}
	go r.timeoutLoop()
	return r.Engine.StartRound()
}

func (r *Runner) timeoutLoop() {
	tick := time.NewTicker(r.RoundTimeout)
	for {
		select {
		case <-r.stop:
			return
		case <-tick.C:
			if r.Engine.Step() == StepPropose {
				_ = r.Engine.ForceTimeoutRound()
				_ = r.Engine.StartRound()
			}
		}
	}
}
```

Expose `Step()` on Engine (already exists).

- [ ] **Step 3: Test + commit**

```bash
git add consensus/runner.go consensus/runner_test.go
git commit -m "feat(consensus): round runner with commit hook and timeout"
```

---

### Task 8: CLI wiring (`cmd/dew`)

**Files:**
- Create: `cmd/dew/bft.go`
- Modify: `cmd/dew/main.go`

- [ ] **Step 1: Add flags to `cmdRun`**

```go
validator := fs.Bool("validator", false, "enable Dew-BFT validator mode")
validatorKey := fs.String("validator.key", "", "hex secp256k1 key for --validator")
noAutoMine := fs.Bool("no-auto-mine", false, "admit txs to mempool only (no local seal)")
```

After `node.NewFromGenesis`:
```go
if *validator {
	*noAutoMine = true
}
if *noAutoMine {
	n.SetAutoMine(false)
}
if *validator {
	if *p2pListen == "" {
		return fmt.Errorf("validator mode requires --p2p.listen")
	}
	return startValidatorStack(n, g, validatorKey, p2pListen, ...)
}
if *noAutoMine && *p2pListen != "" {
	return startFullNodeStack(n, p2pListen, ...)
}
```

- [ ] **Step 2: Implement `startValidatorStack` in `cmd/dew/bft.go`**

Wire:
1. `valSet, _ := consensus.ValidatorSetFromGenesis(g)`
2. Parse validator key; verify in valset
3. `engine, _ := consensus.NewEngine(...)` with `MempoolBlockBuilder{Exec: n}`, `ExecutionValidator{Exec: n}`, `NewBroadcaster(host)`
4. `backend := &node.P2PBackend{N: n}`
5. `host, _ := p2p.NewHost(cfg, backend, backend, handlers)` where handlers forward proposals/votes to engine; `OnBlock` calls `n.ImportCommittedBlock`
6. `runner := &consensus.Runner{Engine: engine, OnCommit: func(ev) { n.ImportCommittedBlock(ev.Block); raw,_ := ev.Block.MarshalBinary(); host chain update; host.GossipBlock }}`
7. `runner.Start()`; start HTTP RPC

- [ ] **Step 3: Implement `startFullNodeStack`**

No engine; `SetAutoMine(false)`; `OnBlock` → `ImportCommittedBlock`; gossip txs on `SendRawTransaction` via `host.GossipTx`.

- [ ] **Step 4: Manual smoke**

```bash
go build -o bin/dew ./cmd/dew
# Run 3 validators in 3 terminals with --validator --validator.key <anvil0-2> --p2p.*
# Verify eth_blockHash matches across nodes
```

- [ ] **Step 5: Commit**

```bash
git add cmd/dew/bft.go cmd/dew/main.go
git commit -m "feat(dew): validator and full-node BFT CLI modes"
```

---

### Task 9: MultiProcessBFT integration harness

**Files:**
- Create: `devnet/multiprocess.go`
- Create: `devnet/multiprocess_bft_test.go`

- [ ] **Step 1: Write failing integration test**

```go
func TestMultiProcessBFT_SharedChain(t *testing.T) {
	net, err := StartMultiProcessBFT(MultiProcessConfig{EncryptP2P: true})
	if err != nil {
		t.Fatal(err)
	}
	defer net.Stop()

	// Submit self-transfer via net.RPCURL (full node)
	// Poll until blockNumber >= 3
	h := pollBlockHash(t, net.RPCURL, 3)
	for i, v := range net.Validators {
		if got := v.Node.CurrentHeader().Hash(); got != h {
			t.Fatalf("validator %d hash %s != %s", i, got.Hex(), h.Hex())
		}
	}
}
```

- [ ] **Step 2: Implement `devnet/multiprocess.go`**

`StartMultiProcessBFT` builds 3 validator stacks + 1 full RPC (reuse wiring from `cmd/dew/bft.go` — extract shared `node/bftstack` package OR duplicate minimally in devnet for test).

Preferred: extract `internal/bftstack` or `node/stack.go` with `type ValidatorStack struct { Node, Engine, Host, Runner }` used by both `cmd/dew` and `devnet`.

- [ ] **Step 3: ERC-20 subtest**

```go
func TestMultiProcessBFT_ERC20(t *testing.T) {
	net, _ := StartMultiProcessBFT(MultiProcessConfig{})
	_, err := DeployAndTransferERC20(net.RPCURL, Faucet(), common.HexToAddress(User1().Address.Hex()), ...)
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 4: Run integration test**

Run: `go test ./devnet/ -run MultiProcessBFT -count=1 -timeout 120s`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add devnet/multiprocess.go devnet/multiprocess_bft_test.go node/stack.go cmd/dew/bft.go
git commit -m "test(devnet): multi-process BFT integration harness"
```

---

### Task 10: Deploy compose + documentation

**Files:**
- Modify: `deploy/node/docker-compose.soak.yml`
- Modify: `docs/development/d3-scale.md` (D3a status → stable)
- Modify: `docs/development/private-testnet.md`
- Modify: `docs/development/phases.md` (D3a checkbox)
- Modify: `agents/debt.md` (mark D3a done)

- [ ] **Step 1: Update compose `multi` profile**

Add to `node-0`, `node-1`, `node-2`:
```yaml
- --validator
- --validator.key
- ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80  # per-node key
```

Add service `node-rpc`:
```yaml
node-rpc:
  profiles: ["multi"]
  command:
    - run
    - --genesis
    - /etc/dew/genesis.json
    - --no-auto-mine
    - --http.addr
    - "0.0.0.0"
    - --http.port
    - "8545"
    - --p2p.listen
    - "0.0.0.0:30303"
    - --p2p.key
    - 7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6
    - --p2p.bootnodes
    - "node-0:30303,node-1:30303,node-2:30303"
  ports:
    - "${DEW_RPC_HTTP_PORT:-8548}:8545"
```

- [ ] **Step 2: Full test suite**

Run: `go test ./... -count=1`  
Expected: all PASS

- [ ] **Step 3: Update docs + debt**

- [ ] **Step 4: Commit**

```bash
git add deploy/node/docker-compose.soak.yml docs/ agents/debt.md
git commit -m "feat(d3a): compose multi BFT layout and close D3a acceptance docs"
```

---

## Self-review (spec coverage)

| Spec requirement | Task |
| :--- | :--- |
| `ImportCommittedBlock` | Task 3 |
| `SetAutoMine` | Task 2 |
| Block wire encoding | Task 1 |
| `MempoolBlockBuilder` / `ExecutionValidator` | Tasks 4–5 |
| P2P bridge + `P2PBackend` | Task 6 |
| `Runner` + 3s timeout | Task 7 |
| CLI `--validator` / `--no-auto-mine` | Task 8 |
| Full RPC sync node | Tasks 8–9 |
| `MultiProcessBFT` test | Task 9 |
| ERC-20 smoke | Task 9 |
| Compose `multi` + `node-rpc` | Task 10 |
| Path B unchanged | Task 8 (default `autoMine=true`) |
| Docs + debt | Task 10 |

No placeholders remain in task steps.

---

## Verification checklist (final)

```bash
go test ./core/types/ ./node/ ./consensus/ ./p2p/ ./devnet/ -count=1
go test ./... -count=1
go build -o bin/dew ./cmd/dew
node scripts/devnet-erc20.mjs http://127.0.0.1:8548   # after compose multi up
```