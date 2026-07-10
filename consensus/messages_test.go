package consensus

import (
	"testing"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func TestSignVerifyProposalAndVote(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hash := types.Keccak256Hash([]byte("block-1"))
	p := &Proposal{Height: 1, Round: 0, BlockHash: hash}
	if err := SignProposal(p, key); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProposal(p); err != nil {
		t.Fatal(err)
	}

	v := &Vote{Type: VotePrevote, Height: 1, Round: 0, BlockHash: hash}
	if err := SignVote(v, key); err != nil {
		t.Fatal(err)
	}
	if err := VerifyVote(v); err != nil {
		t.Fatal(err)
	}

	// Tamper
	v.BlockHash = types.Keccak256Hash([]byte("other"))
	if err := VerifyVote(v); err == nil {
		t.Fatal("expected verify fail on tampered vote")
	}
}
