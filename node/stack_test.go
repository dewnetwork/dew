package node_test

import (
	"testing"
	"time"

	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/p2p"
)

func TestStack_TwoValidatorProposalDelivery(t *testing.T) {
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	vals := devnet.DefaultValidators()

	start := func(i int, boots []string) (*node.Stack, error) {
		n := node.OpenTest(t, g)
		return node.StartStack(node.StackConfig{
			Genesis:        g,
			Node:           n,
			Validator:      true,
			ValidatorKey:   vals[i].PrivateKey,
			P2PListen:      "127.0.0.1:0",
			P2PPrivateKey:  vals[i].PrivateKey,
			Bootnodes:      boots,
			Encrypt:        true,
			AllowCleartext: false,
			DeferRunner:    true,
		})
	}

	v0, err := start(0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v0.Stop()

	v1, err := start(1, []string{v0.Host.ListenAddr()})
	if err != nil {
		t.Fatal(err)
	}
	defer v1.Stop()

	time.Sleep(200 * time.Millisecond)

	v2, err := start(2, []string{v0.Host.ListenAddr(), v1.Host.ListenAddr()})
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Stop()
	stacks := []*node.Stack{v0, v1, v2}
	for i, a := range stacks {
		for j, b := range stacks {
			if i == j {
				continue
			}
			if _, ok := a.Host.Store().GetActive(b.Host.ID()); ok {
				continue
			}
			if _, err := a.Host.Dial(b.Host.ListenAddr()); err != nil {
				t.Logf("dial %d→%d: %v", i, j, err)
			}
		}
	}
	time.Sleep(300 * time.Millisecond)
	for _, s := range stacks {
		if err := s.StartConsensus(); err != nil {
			t.Fatal(err)
		}
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if v0.Node.BlockNumber() >= 3 && v1.Node.BlockNumber() >= 3 && v2.Node.BlockNumber() >= 3 {
			return
		}
		t.Logf("v0 step=%s node=%d peers=%d | v1 step=%s node=%d peers=%d | v2 step=%s node=%d peers=%d",
			v0.Engine.Step(), v0.Node.BlockNumber(), v0.Host.PeerCount(),
			v1.Engine.Step(), v1.Node.BlockNumber(), v1.Host.PeerCount(),
			v2.Engine.Step(), v2.Node.BlockNumber(), v2.Host.PeerCount())
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("no commit: v0=%d v1=%d", v0.Node.BlockNumber(), v1.Node.BlockNumber())
}

func TestStack_WireProposalRoundTrip(t *testing.T) {
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	n := node.OpenTest(t, g)
	vals := devnet.DefaultValidators()
	valSet, err := consensus.ValidatorSetFromGenesis(g)
	if err != nil {
		t.Fatal(err)
	}
	parent := n.CurrentHeader()
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, vals[1].Address, 1)
	if err != nil {
		t.Fatal(err)
	}
	p := &consensus.Proposal{
		Height:    parent.Number + 1,
		Round:     0,
		BlockHash: blk.Hash(),
		Block:     blk,
		Proposer:  vals[1].Address,
	}
	if err := consensus.SignProposal(p, vals[1].PrivateKey); err != nil {
		t.Fatal(err)
	}
	wp, err := p2p.ProposalToWire(p)
	if err != nil {
		t.Fatal(err)
	}
	back, err := p2p.WireToProposal(wp)
	if err != nil {
		t.Fatal(err)
	}
	if err := consensus.VerifyProposal(back); err != nil {
		t.Fatal(err)
	}
	eng, err := consensus.NewEngine(consensus.EngineConfig{
		PrivateKey:  vals[0].PrivateKey,
		ValSet:      valSet,
		Parent:      parent,
		Builder:     &consensus.MempoolBlockBuilder{Exec: n, MaxTxs: 1},
		Validator:   &consensus.ExecutionValidator{Exec: n},
		ProposeRoot: parent.StateRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.HandleProposal(back); err != nil {
		t.Fatalf("handle proposal: %v", err)
	}
	if eng.Step() != consensus.StepPrevote {
		t.Fatalf("step=%s want prevote", eng.Step())
	}
}