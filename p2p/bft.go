package p2p

import (
	"fmt"

	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/types"
)

// ProposalToWire maps a consensus proposal to the P2P wire form.
// BlockRaw is populated from block.MarshalBinary when p.Block is set.
func ProposalToWire(p *consensus.Proposal) (*WireProposal, error) {
	if p == nil {
		return nil, fmt.Errorf("p2p: nil proposal")
	}
	wp := &WireProposal{
		Height:    p.Height,
		Round:     p.Round,
		BlockHash: p.BlockHash,
		Proposer:  p.Proposer,
		Signature: append([]byte(nil), p.Signature...),
	}
	if p.Block != nil {
		raw, err := p.Block.MarshalBinary()
		if err != nil {
			return nil, err
		}
		wp.BlockRaw = raw
	}
	return wp, nil
}

// WireToProposal decodes a wire proposal into consensus form.
// Non-empty BlockRaw is unmarshalled into Proposal.Block.
func WireToProposal(wp *WireProposal) (*consensus.Proposal, error) {
	if wp == nil {
		return nil, fmt.Errorf("p2p: nil wire proposal")
	}
	p := &consensus.Proposal{
		Height:    wp.Height,
		Round:     wp.Round,
		BlockHash: wp.BlockHash,
		Proposer:  wp.Proposer,
		Signature: append([]byte(nil), wp.Signature...),
	}
	if len(wp.BlockRaw) > 0 {
		blk, err := types.UnmarshalBlockBinary(wp.BlockRaw)
		if err != nil {
			return nil, err
		}
		p.Block = blk
	}
	return p, nil
}

// VoteToWire maps a consensus vote to the P2P wire form.
// VotePrevote=1, VotePrecommit=2.
func VoteToWire(v *consensus.Vote) (*WireVote, error) {
	if v == nil {
		return nil, fmt.Errorf("p2p: nil vote")
	}
	var typ uint8
	switch v.Type {
	case consensus.VotePrevote:
		typ = 1
	case consensus.VotePrecommit:
		typ = 2
	default:
		return nil, fmt.Errorf("p2p: invalid vote type %d", v.Type)
	}
	return &WireVote{
		Type:      typ,
		Height:    v.Height,
		Round:     v.Round,
		BlockHash: v.BlockHash,
		Validator: v.Validator,
		Signature: append([]byte(nil), v.Signature...),
	}, nil
}

// WireToVote decodes a wire vote into consensus form.
func WireToVote(wv *WireVote) (*consensus.Vote, error) {
	if wv == nil {
		return nil, fmt.Errorf("p2p: nil wire vote")
	}
	var typ consensus.VoteType
	switch wv.Type {
	case 1:
		typ = consensus.VotePrevote
	case 2:
		typ = consensus.VotePrecommit
	default:
		return nil, fmt.Errorf("p2p: invalid wire vote type %d", wv.Type)
	}
	return &consensus.Vote{
		Type:      typ,
		Height:    wv.Height,
		Round:     wv.Round,
		BlockHash: wv.BlockHash,
		Validator: wv.Validator,
		Signature: append([]byte(nil), wv.Signature...),
	}, nil
}

type p2pBroadcaster struct {
	host *Host
}

// NewBroadcaster returns a consensus.Broadcaster that floods proposals and votes via Host.
func NewBroadcaster(h *Host) consensus.Broadcaster {
	return &p2pBroadcaster{host: h}
}

func (b *p2pBroadcaster) BroadcastProposal(p *consensus.Proposal) {
	wp, err := ProposalToWire(p)
	if err != nil {
		return
	}
	_ = b.host.BroadcastProposal(wp)
}

func (b *p2pBroadcaster) BroadcastVote(v *consensus.Vote) {
	wv, err := VoteToWire(v)
	if err != nil {
		return
	}
	_ = b.host.BroadcastVote(wv)
}