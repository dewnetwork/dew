package consensus

import (
	"testing"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// TestSingleValidatorAutoCommit — Phase A5 acceptance: single validator auto-commits.
func TestSingleValidatorAutoCommit(t *testing.T) {
	root := types.Keccak256Hash([]byte("state-v1"))
	nodes, err := GenerateLocalNodes(1, 1, root)
	if err != nil {
		t.Fatal(err)
	}
	parent := MakeGenesisHeader()
	cluster, err := NewLocalCluster(parent, nodes)
	if err != nil {
		t.Fatal(err)
	}

	ev, err := cluster.RunHeight()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Height != 1 {
		t.Fatalf("height=%d want 1", ev.Height)
	}
	if ev.BlockHash.IsZero() {
		t.Fatal("empty block hash")
	}
	if ev.Block == nil {
		t.Fatal("nil block")
	}
	if ev.Block.Header().StateRoot != root {
		t.Fatalf("state root mismatch")
	}
	if h := cluster.Engines()[0].Height(); h != 2 {
		t.Fatalf("engine height=%d want 2", h)
	}
}

// TestRoundStateMachineAdvancesHeight — multiple sequential commits.
func TestRoundStateMachineAdvancesHeight(t *testing.T) {
	root := types.Keccak256Hash([]byte("adv"))
	nodes, err := GenerateLocalNodes(1, 10, root)
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := NewLocalCluster(MakeGenesisHeader(), nodes)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := cluster.RunHeights(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 5 {
		t.Fatalf("got %d commits", len(evs))
	}
	for i, ev := range evs {
		want := uint64(i + 1)
		if ev.Height != want {
			t.Fatalf("commit[%d].Height=%d want %d", i, ev.Height, want)
		}
		if i > 0 {
			if ev.Block.Header().ParentHash != evs[i-1].BlockHash {
				t.Fatalf("height %d parent link broken", ev.Height)
			}
		}
	}
	if cluster.Engines()[0].Height() != 6 {
		t.Fatalf("want height 6 after 5 commits, got %d", cluster.Engines()[0].Height())
	}
}

// TestThreeValidatorsCommitQuorum — Phase A5: 3 local validators reach commit with >2/3.
func TestThreeValidatorsCommitQuorum(t *testing.T) {
	root := types.Keccak256Hash([]byte("tri"))
	nodes, err := GenerateLocalNodes(3, 1, root)
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := NewLocalCluster(MakeGenesisHeader(), nodes)
	if err != nil {
		t.Fatal(err)
	}
	if cluster.ValSet().Size() != 3 {
		t.Fatalf("valset size %d", cluster.ValSet().Size())
	}

	ev, err := cluster.RunHeight()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Height != 1 {
		t.Fatalf("height %d", ev.Height)
	}
	var power uint64
	seen := map[crypto.Address]struct{}{}
	for _, v := range ev.Precommits {
		if _, ok := seen[v.Validator]; ok {
			continue
		}
		seen[v.Validator] = struct{}{}
		power += cluster.ValSet().PowerOf(v.Validator)
	}
	if !cluster.ValSet().HasQuorum(power) {
		t.Fatalf("precommit power %d not quorum of %d", power, cluster.ValSet().TotalPower())
	}
	for i, eng := range cluster.Engines() {
		if eng.Height() != 2 {
			t.Fatalf("engine[%d] height=%d want 2", i, eng.Height())
		}
		if eng.LastCommit == nil || eng.LastCommit.BlockHash != ev.BlockHash {
			t.Fatalf("engine[%d] disagree on commit", i)
		}
	}

	if _, err := cluster.RunHeights(3); err != nil {
		t.Fatal(err)
	}
	if cluster.Engines()[0].Height() != 5 {
		t.Fatalf("want height 5, got %d", cluster.Engines()[0].Height())
	}
}

// TestApplySyncedBlock_CatchUp advances a lagging engine after P2P import.
func TestApplySyncedBlock_CatchUp(t *testing.T) {
	root := types.Keccak256Hash([]byte("sync-catchup"))
	nodes, err := GenerateLocalNodes(1, 1, root)
	if err != nil {
		t.Fatal(err)
	}
	parent := MakeGenesisHeader()
	cluster, err := NewLocalCluster(parent, nodes)
	if err != nil {
		t.Fatal(err)
	}
	eng := cluster.Engines()[0]
	if eng.Height() != 1 {
		t.Fatalf("height=%d want 1", eng.Height())
	}

	// Simulate a block committed by peers while this engine lagged at height 1.
	blk := types.NewBlock(&types.Header{
		ParentHash: parent.Hash(),
		Number:     1,
		Timestamp:  1,
		GasLimit:   parent.GasLimit,
		StateRoot:  root,
		TxRoot:     types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Proposer:   eng.Address(),
	}, nil)

	advanced, err := eng.ApplySyncedBlock(blk)
	if err != nil {
		t.Fatal(err)
	}
	if !advanced {
		t.Fatal("expected engine to advance")
	}
	if eng.Height() != 2 {
		t.Fatalf("height=%d want 2", eng.Height())
	}
	if eng.Step() != StepNewRound {
		t.Fatalf("step=%s want NewRound", eng.Step())
	}

	// Idempotent once past this height.
	advanced, err = eng.ApplySyncedBlock(blk)
	if err != nil {
		t.Fatal(err)
	}
	if advanced {
		t.Fatal("expected no-op after catch-up")
	}
}

// TestInvalidRootPrevoteNil — Phase A5: invalid state root → prevote nil → no commit.
func TestInvalidRootPrevoteNil(t *testing.T) {
	goodRoot := types.Keccak256Hash([]byte("good"))
	badRoot := types.Keccak256Hash([]byte("bad"))

	var nodes []LocalNode
	for i := 0; i < 3; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		// Proposers claim badRoot; validators only accept goodRoot → nil prevotes.
		nodes = append(nodes, LocalNode{
			Key:         key,
			Power:       1,
			ProposeRoot: badRoot,
			Validator:   NewRootValidator(goodRoot),
		})
	}
	cluster, err := NewLocalCluster(MakeGenesisHeader(), nodes)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.RunHeight()
	if err == nil {
		t.Fatal("expected commit failure for invalid root")
	}
	for i, eng := range cluster.Engines() {
		if eng.Height() != 1 {
			t.Fatalf("engine[%d] height=%d want 1 (no commit)", i, eng.Height())
		}
		if eng.LastCommit != nil {
			t.Fatalf("engine[%d] unexpectedly committed", i)
		}
		// Nil polka advances round without committing.
		if eng.Round() < 1 {
			t.Fatalf("engine[%d] round=%d want >=1 after nil polka", i, eng.Round())
		}
	}

	// Fix roots so a later StartRound can commit.
	for _, eng := range cluster.Engines() {
		eng.SetProposeRoot(goodRoot)
		eng.SetValidator(NewRootValidator(goodRoot))
	}
	ev, err := cluster.RunHeight()
	if err != nil {
		t.Fatalf("expected commit after fixing roots: %v", err)
	}
	if ev.Block.Header().StateRoot != goodRoot {
		t.Fatalf("committed bad root")
	}
}
