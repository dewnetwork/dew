package p2p

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/dewnetwork/dew/core/types"
)

func (h *Host) handleFrame(p *Peer, f Frame) error {
	switch f.Type {
	case MsgHandshake:
		// Only valid during negotiate; ignore if late.
		return nil
	case MsgPing:
		return p.Send(MsgPong, f.Payload)
	case MsgPong:
		return nil
	case MsgGetPeers:
		return h.handleGetPeers(p)
	case MsgPeers:
		return h.handlePeers(f.Payload)
	case MsgInventory:
		return h.handleInventory(p, f.Payload)
	case MsgGetData:
		return h.handleGetData(p, f.Payload)
	case MsgTxPayload:
		return h.handleTxPayload(p, f.Payload)
	case MsgBlockPayload:
		return h.handleBlockPayload(p, f.Payload)
	case MsgGetBlocks:
		return h.handleGetBlocks(p, f.Payload)
	case MsgBlocks:
		return h.handleBlocks(p, f.Payload)
	case MsgProposal:
		return h.handleProposal(p, f.Payload)
	case MsgPrevote, MsgPrecommit:
		return h.handleVote(p, f.Payload, f.Type)
	default:
		// Unknown types ignored for forward compatibility.
		return nil
	}
}

func (h *Host) handleGetPeers(p *Peer) error {
	known := h.store.Known()
	msg := &PeersMsg{Peers: make([]PeerAddr, 0, len(known))}
	for _, k := range known {
		host, port, err := splitHostPort(k.Addr)
		if err != nil {
			continue
		}
		msg.Peers = append(msg.Peers, PeerAddr{ID: k.ID, Host: host, Port: port})
	}
	payload, err := msg.Encode()
	if err != nil {
		return err
	}
	return p.Send(MsgPeers, payload)
}

func (h *Host) handlePeers(payload []byte) error {
	msg, err := DecodePeersMsg(payload)
	if err != nil {
		return err
	}
	for _, pa := range msg.Peers {
		if pa.ID.Equal(h.nodeID) {
			continue
		}
		h.store.Remember(pa.ID, FormatAddr(pa.Host, pa.Port))
	}
	return nil
}

func (h *Host) handleInventory(p *Peer, payload []byte) error {
	inv, err := DecodeInventory(payload)
	if err != nil {
		return err
	}
	var missing []InvItem
	for _, it := range inv.Items {
		if h.alreadySeen(it.Hash) {
			continue
		}
		switch it.Kind {
		case InvTx:
			if h.txs.HasTx(it.Hash) {
				continue
			}
		case InvBlock:
			if h.chain.HasBlock(it.Hash) {
				continue
			}
		default:
			continue
		}
		missing = append(missing, it)
		h.markSeen(it.Hash)
	}
	if len(missing) == 0 {
		return nil
	}
	req := &GetData{Items: missing}
	enc, err := req.Encode()
	if err != nil {
		return err
	}
	return p.Send(MsgGetData, enc)
}

func (h *Host) handleGetData(p *Peer, payload []byte) error {
	req, err := DecodeGetData(payload)
	if err != nil {
		return err
	}
	for _, it := range req.Items {
		switch it.Kind {
		case InvTx:
			raw, ok := h.txs.GetTx(it.Hash)
			if !ok {
				continue
			}
			msg := &TxPayload{Hash: it.Hash, Raw: raw}
			enc, err := msg.Encode()
			if err != nil {
				return err
			}
			if err := p.Send(MsgTxPayload, enc); err != nil {
				return err
			}
		case InvBlock:
			raw, num, ok := h.chain.BlockByHash(it.Hash)
			if !ok {
				continue
			}
			msg := &BlockPayload{Number: num, Hash: it.Hash, Raw: raw}
			enc, err := msg.Encode()
			if err != nil {
				return err
			}
			if err := p.Send(MsgBlockPayload, enc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Host) handleTxPayload(p *Peer, payload []byte) error {
	msg, err := DecodeTxPayload(payload)
	if err != nil {
		return err
	}
	if h.txs.HasTx(msg.Hash) {
		return nil
	}
	if h.handlers.OnTx != nil {
		if err := h.handlers.OnTx(msg.Hash, msg.Raw, p.ID); err != nil {
			return nil // drop invalid without killing peer
		}
	}
	inv := &Inventory{Items: []InvItem{{Kind: InvTx, Hash: msg.Hash}}}
	enc, err := inv.Encode()
	if err != nil {
		return err
	}
	h.Broadcast(MsgInventory, enc, p.ID)
	return nil
}

func (h *Host) handleBlockPayload(p *Peer, payload []byte) error {
	msg, err := DecodeBlockPayload(payload)
	if err != nil {
		return err
	}
	if h.chain.HasBlock(msg.Hash) {
		return nil
	}
	if h.handlers.OnBlock != nil {
		if err := h.handlers.OnBlock(msg.Number, msg.Hash, msg.Raw, p.ID); err != nil {
			// Import may fail when we are behind; allow inventory re-fetch.
			h.unmarkSeen(msg.Hash)
			return nil
		}
	}
	if msg.Number > p.Height {
		p.Height = msg.Number
		p.HeadHash = msg.Hash
	}
	inv := &Inventory{Items: []InvItem{{Kind: InvBlock, Hash: msg.Hash}}}
	enc, err := inv.Encode()
	if err != nil {
		return err
	}
	h.Broadcast(MsgInventory, enc, p.ID)
	return nil
}

func (h *Host) handleGetBlocks(p *Peer, payload []byte) error {
	req, err := DecodeGetBlocks(payload)
	if err != nil {
		return err
	}
	if req.To < req.From {
		return fmt.Errorf("p2p: invalid block range")
	}
	if req.To-req.From+1 > h.cfg.MaxBlocksPerRequest {
		req.To = req.From + h.cfg.MaxBlocksPerRequest - 1
	}
	msg := &BlocksMsg{}
	for n := req.From; n <= req.To; n++ {
		raw, hash, ok := h.chain.BlockByNumber(n)
		if !ok {
			break
		}
		msg.Blocks = append(msg.Blocks, BlockPayload{Number: n, Hash: hash, Raw: raw})
	}
	enc, err := msg.Encode()
	if err != nil {
		return err
	}
	return p.Send(MsgBlocks, enc)
}

func (h *Host) handleBlocks(p *Peer, payload []byte) error {
	msg, err := DecodeBlocksMsg(payload)
	if err != nil {
		return err
	}
	for _, bl := range msg.Blocks {
		if h.chain.HasBlock(bl.Hash) {
			continue
		}
		if h.handlers.OnBlock != nil {
			_ = h.handlers.OnBlock(bl.Number, bl.Hash, bl.Raw, p.ID)
		}
	}
	return nil
}

func (h *Host) handleProposal(p *Peer, payload []byte) error {
	msg, err := DecodeWireProposal(payload)
	if err != nil {
		return err
	}
	// Dedup proposals without colliding with block inventory seen keys.
	propKey := proposalSeenHash(msg)
	if h.alreadySeen(propKey) {
		return nil
	}
	h.markSeen(propKey)
	if h.handlers.OnProposal != nil {
		if err := h.handlers.OnProposal(msg, p.ID); err != nil {
			return nil
		}
	}
	h.Broadcast(MsgProposal, payload, p.ID)
	return nil
}

func proposalSeenHash(msg *WireProposal) types.Hash {
	var b [8 + 8 + 32]byte
	binary.BigEndian.PutUint64(b[0:8], msg.Height)
	binary.BigEndian.PutUint64(b[8:16], msg.Round)
	copy(b[16:], msg.BlockHash[:])
	return types.Keccak256Hash(b[:])
}

func (h *Host) handleVote(p *Peer, payload []byte, typ uint8) error {
	msg, err := DecodeWireVote(payload)
	if err != nil {
		return err
	}
	// Dedup votes by (type,height,round,validator) fingerprint.
	voteKey := voteSeenHash(msg)
	if h.alreadySeen(voteKey) {
		return nil
	}
	h.markSeen(voteKey)
	if h.handlers.OnVote != nil {
		if err := h.handlers.OnVote(msg, p.ID); err != nil {
			return nil
		}
	}
	h.Broadcast(typ, payload, p.ID)
	return nil
}

func voteSeenHash(msg *WireVote) types.Hash {
	// Compact fingerprint for inventory-style seen set.
	var b [1 + 8 + 8 + 20]byte
	b[0] = msg.Type
	binary.BigEndian.PutUint64(b[1:9], msg.Height)
	binary.BigEndian.PutUint64(b[9:17], msg.Round)
	copy(b[17:], msg.Validator[:])
	return types.Keccak256Hash(b[:])
}

func (h *Host) alreadySeen(hash types.Hash) bool {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	_, ok := h.seenInv[hash]
	return ok
}

func (h *Host) markSeen(hash types.Hash) {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	h.seenInv[hash] = struct{}{}
}

func (h *Host) unmarkSeen(hash types.Hash) {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	delete(h.seenInv, hash)
}

func splitHostPort(addr string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, err
	}
	return host, uint16(p), nil
}
