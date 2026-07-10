package p2p

import (
	"fmt"

	"github.com/dewnetwork/dew/core/types"
)

// GossipTx announces a transaction hash to all peers (inventory).
// The local node should already store the tx in TxBackend (via handlers/app).
func (h *Host) GossipTx(hash types.Hash) error {
	inv := &Inventory{Items: []InvItem{{Kind: InvTx, Hash: hash}}}
	enc, err := inv.Encode()
	if err != nil {
		return err
	}
	h.markSeen(hash)
	h.Broadcast(MsgInventory, enc, PeerID{})
	return nil
}

// GossipBlock announces a block hash to all peers.
func (h *Host) GossipBlock(hash types.Hash) error {
	inv := &Inventory{Items: []InvItem{{Kind: InvBlock, Hash: hash}}}
	enc, err := inv.Encode()
	if err != nil {
		return err
	}
	h.markSeen(hash)
	h.Broadcast(MsgInventory, enc, PeerID{})
	return nil
}

// BroadcastProposal floods a consensus proposal to all peers.
func (h *Host) BroadcastProposal(msg *WireProposal) error {
	if msg == nil {
		return fmt.Errorf("p2p: nil proposal")
	}
	enc, err := msg.Encode()
	if err != nil {
		return err
	}
	h.Broadcast(MsgProposal, enc, PeerID{})
	return nil
}

// BroadcastVote floods a consensus vote (prevote or precommit).
func (h *Host) BroadcastVote(msg *WireVote) error {
	if msg == nil {
		return fmt.Errorf("p2p: nil vote")
	}
	var typ uint8
	switch msg.Type {
	case 1:
		typ = MsgPrevote
	case 2:
		typ = MsgPrecommit
	default:
		return fmt.Errorf("p2p: invalid vote type %d", msg.Type)
	}
	enc, err := msg.Encode()
	if err != nil {
		return err
	}
	h.Broadcast(typ, enc, PeerID{})
	return nil
}

// RequestPeers asks a peer for its known address book (PEX).
func (h *Host) RequestPeers(p *Peer) error {
	return p.Send(MsgGetPeers, nil)
}
