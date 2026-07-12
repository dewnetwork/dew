package p2p

import (
	"fmt"
	"time"
)

// SyncFromPeer requests blocks from the peer until local height catches the
// peer tip (or the peer has nothing more). Blocks are applied via
// AppHandlers.OnBlock; the backend must store them so Height() advances.
//
// When the local chain is empty (no block 0), requests start at height 0.
// When genesis is already present, requests start at localHeight+1.
func (h *Host) SyncFromPeer(p *Peer) error {
	if p == nil {
		return fmt.Errorf("p2p: nil peer")
	}
	for {
		local := h.chain.Height()
		target := p.Height
		if target < local {
			return nil
		}
		// Caught up when we have the same tip height and at least one block known,
		// or peer tip equals local and we hold that height.
		if target == local {
			if _, _, ok := h.chain.BlockByNumber(local); ok {
				return nil
			}
		}

		from := nextSyncFrom(h.chain)
		if from > target {
			return nil
		}
		to := from + h.cfg.MaxBlocksPerRequest - 1
		if to > target {
			to = target
		}

		req := &GetBlocks{From: from, To: to}
		enc, err := req.Encode()
		if err != nil {
			return err
		}
		beforeH := h.chain.Height()
		beforeHas := countBlocks(h.chain, from, to)
		if err := p.Send(MsgGetBlocks, enc); err != nil {
			return err
		}

		deadline := time.Now().Add(5 * time.Second)
		progressed := false
		for time.Now().Before(deadline) {
			if h.chain.Height() > beforeH || countBlocks(h.chain, from, to) > beforeHas {
				progressed = true
				break
			}
			// Fully caught up mid-wait
			if h.chain.Height() >= target {
				if _, _, ok := h.chain.BlockByNumber(target); ok {
					return nil
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !progressed {
			return fmt.Errorf("p2p: sync stall at height %d (want through %d, peer tip %d)", beforeH, to, target)
		}
	}
}

// nextSyncFrom returns the first height we still need.
func nextSyncFrom(chain ChainBackend) uint64 {
	// Find lowest missing from 0..height, else height+1.
	h := chain.Height()
	for n := uint64(0); n <= h; n++ {
		if _, _, ok := chain.BlockByNumber(n); !ok {
			return n
		}
	}
	// All 0..h present → request next.
	return h + 1
}

func countBlocks(chain ChainBackend, from, to uint64) int {
	n := 0
	for i := from; i <= to; i++ {
		if _, _, ok := chain.BlockByNumber(i); ok {
			n++
		}
	}
	return n
}

// SyncMissingFromPeer requests blocks from local height+1 without trusting peer.Height
// (handshake height can be stale on long-lived connections).
func (h *Host) SyncMissingFromPeer(p *Peer) error {
	if p == nil {
		return fmt.Errorf("p2p: nil peer")
	}
	const maxBatches = 32
	for batch := 0; batch < maxBatches; batch++ {
		from := nextSyncFrom(h.chain)
		to := from + h.cfg.MaxBlocksPerRequest - 1
		req := &GetBlocks{From: from, To: to}
		enc, err := req.Encode()
		if err != nil {
			return err
		}
		beforeH := h.chain.Height()
		beforeHas := countBlocks(h.chain, from, to)
		if err := p.Send(MsgGetBlocks, enc); err != nil {
			return err
		}
		deadline := time.Now().Add(2 * time.Second)
		progressed := false
		for time.Now().Before(deadline) {
			if h.chain.Height() > beforeH || countBlocks(h.chain, from, to) > beforeHas {
				progressed = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !progressed {
			return nil
		}
	}
	return nil
}

// SyncBestPeer picks the highest peer and syncs from it.
func (h *Host) SyncBestPeer() error {
	var best *Peer
	local := h.chain.Height()
	for _, p := range h.store.Active() {
		if p.Height > local && (best == nil || p.Height > best.Height) {
			best = p
		}
	}
	if best == nil {
		// Also try peers at same height if we are missing genesis/history.
		for _, p := range h.store.Active() {
			if best == nil || p.Height > best.Height {
				best = p
			}
		}
		if best == nil {
			return nil
		}
		// If we already have full chain to best.Height, done.
		if h.chain.Height() >= best.Height {
			if _, _, ok := h.chain.BlockByNumber(best.Height); ok {
				return nil
			}
		}
	}
	return h.SyncFromPeer(best)
}
