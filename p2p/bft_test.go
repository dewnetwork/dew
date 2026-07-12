package p2p

import (
	"testing"

	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func TestProposalWireRoundTrip(t *testing.T) {
	p := &consensus.Proposal{
		Height:    1,
		Round:     0,
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
	if back.Height != p.Height || back.Round != p.Round {
		t.Fatal("height/round mismatch")
	}
	if back.Proposer != p.Proposer {
		t.Fatal("proposer mismatch")
	}
	if len(back.Signature) != len(p.Signature) {
		t.Fatal("signature length mismatch")
	}

	// Block body round-trips via BlockRaw.
	hdr := &types.Header{
		ParentHash:  types.Hash{},
		StateRoot:   types.Keccak256Hash([]byte("st")),
		TxRoot:      types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Number:      1,
		Timestamp:   100,
		GasLimit:    30_000_000,
		GasUsed:     0,
		BaseFee:     nil,
	}
	blk := types.NewBlock(hdr, nil)
	p2 := &consensus.Proposal{
		Height:    2,
		Round:     1,
		BlockHash: blk.Hash(),
		Block:     blk,
		Proposer:  crypto.Address{1},
		Signature: make([]byte, 65),
	}
	wp2, err := ProposalToWire(p2)
	if err != nil {
		t.Fatal(err)
	}
	if len(wp2.BlockRaw) == 0 {
		t.Fatal("expected BlockRaw when proposal has block body")
	}
	back2, err := WireToProposal(wp2)
	if err != nil {
		t.Fatal(err)
	}
	if back2.Block == nil {
		t.Fatal("expected decoded block")
	}
	if back2.Block.Hash() != blk.Hash() {
		t.Fatal("block hash mismatch after round trip")
	}
}

func TestVoteWireRoundTrip(t *testing.T) {
	hash := types.Keccak256Hash([]byte("vote-block"))
	v := &consensus.Vote{
		Type:      consensus.VotePrevote,
		Height:    3,
		Round:     2,
		BlockHash: hash,
		Validator: crypto.Address{2},
		Signature: make([]byte, 65),
	}
	wv, err := VoteToWire(v)
	if err != nil {
		t.Fatal(err)
	}
	if wv.Type != 1 {
		t.Fatalf("prevote wire type: got %d want 1", wv.Type)
	}
	back, err := WireToVote(wv)
	if err != nil {
		t.Fatal(err)
	}
	if back.Type != consensus.VotePrevote {
		t.Fatalf("vote type: got %v want prevote", back.Type)
	}
	if back.BlockHash != hash {
		t.Fatal("block hash mismatch")
	}

	v2 := &consensus.Vote{
		Type:      consensus.VotePrecommit,
		Height:    3,
		Round:     2,
		BlockHash: hash,
		Validator: crypto.Address{3},
		Signature: make([]byte, 65),
	}
	wv2, err := VoteToWire(v2)
	if err != nil {
		t.Fatal(err)
	}
	if wv2.Type != 2 {
		t.Fatalf("precommit wire type: got %d want 2", wv2.Type)
	}
	back2, err := WireToVote(wv2)
	if err != nil {
		t.Fatal(err)
	}
	if back2.Type != consensus.VotePrecommit {
		t.Fatalf("vote type: got %v want precommit", back2.Type)
	}
}