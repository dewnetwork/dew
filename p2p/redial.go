package p2p

import (
	"sync"
	"time"
)

const (
	redialMinBackoff = 1 * time.Second
	redialMaxBackoff = 5 * time.Minute
	redialTick       = 1 * time.Second
	peerSaveDebounce = 1 * time.Second
)

type addrBackoff struct {
	next    time.Time
	backoff time.Duration
}

// redialState tracks per-address dial backoff for the maintain loop.
type redialState struct {
	mu    sync.Mutex
	addrs map[string]*addrBackoff
}

func newRedialState() *redialState {
	return &redialState{addrs: make(map[string]*addrBackoff)}
}

func (r *redialState) shouldDial(addr string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.addrs[addr]
	if !ok {
		return true
	}
	return !now.Before(st.next)
}

func (r *redialState) success(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.addrs, addr)
}

func (r *redialState) failure(addr string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.addrs[addr]
	if !ok {
		st = &addrBackoff{backoff: redialMinBackoff}
		r.addrs[addr] = st
	} else {
		st.backoff *= 2
		if st.backoff > redialMaxBackoff {
			st.backoff = redialMaxBackoff
		}
	}
	st.next = now.Add(st.backoff)
}

// scheduleSoon forces a dial attempt after min backoff (e.g. on disconnect).
func (r *redialState) scheduleSoon(addr string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addrs[addr] = &addrBackoff{
		next:    now.Add(redialMinBackoff),
		backoff: redialMinBackoff,
	}
}

// SetBootnodes replaces the bootnode dial set (always retried, never evicted by TTL).
func (h *Host) SetBootnodes(addrs []string) {
	h.bootMu.Lock()
	defer h.bootMu.Unlock()
	h.bootnodes = make(map[string]struct{}, len(addrs))
	for _, a := range addrs {
		if a == "" {
			continue
		}
		h.bootnodes[a] = struct{}{}
	}
}

// Bootnodes returns a snapshot of configured bootnode addresses.
func (h *Host) Bootnodes() []string {
	h.bootMu.Lock()
	defer h.bootMu.Unlock()
	out := make([]string, 0, len(h.bootnodes))
	for a := range h.bootnodes {
		out = append(out, a)
	}
	return out
}

func (h *Host) bootnodeSet() map[string]struct{} {
	h.bootMu.Lock()
	defer h.bootMu.Unlock()
	out := make(map[string]struct{}, len(h.bootnodes))
	for a := range h.bootnodes {
		out[a] = struct{}{}
	}
	return out
}

func (h *Host) markPeersDirty() {
	h.saveMu.Lock()
	h.peersDirty = true
	h.saveMu.Unlock()
}

func (h *Host) maybeSavePeers(force bool) {
	if h.cfg.PeerStorePath == "" {
		return
	}
	h.saveMu.Lock()
	if !force && !h.peersDirty {
		h.saveMu.Unlock()
		return
	}
	if !force && time.Since(h.lastSave) < peerSaveDebounce {
		h.saveMu.Unlock()
		return
	}
	h.peersDirty = false
	h.lastSave = time.Now()
	path := h.cfg.PeerStorePath
	h.saveMu.Unlock()
	_ = h.store.SaveToFile(path)
}

func (h *Host) startRedialLoop() {
	if !h.cfg.redialEnabled() {
		return
	}
	h.wg.Add(1)
	go h.redialLoop()
}

func (h *Host) redialLoop() {
	defer h.wg.Done()
	ticker := time.NewTicker(redialTick)
	defer ticker.Stop()
	// Immediate pass so bootnodes connect without waiting a full tick.
	h.maintainOnce()
	for {
		select {
		case <-h.redialStop:
			return
		case <-ticker.C:
			if h.closed.Load() {
				return
			}
			h.maintainOnce()
			h.maybeSavePeers(false)
		}
	}
}

func (h *Host) maintainOnce() {
	keep := h.bootnodeSet()
	_ = h.store.EvictOlderThan(DefaultPeerTTL, keep)

	now := time.Now()
	// Collect dial targets: bootnodes + dialable known peers.
	targets := make(map[string]struct{})
	for addr := range keep {
		if !h.addrConnected(addr) {
			targets[addr] = struct{}{}
		}
	}
	for _, k := range h.store.Dialable() {
		if k.Addr == "" {
			continue
		}
		targets[k.Addr] = struct{}{}
	}

	for addr := range targets {
		if h.closed.Load() {
			return
		}
		if !h.redial.shouldDial(addr, now) {
			continue
		}
		if h.addrConnected(addr) {
			h.redial.success(addr)
			continue
		}
		if _, err := h.Dial(addr); err != nil {
			h.redial.failure(addr, now)
			continue
		}
		h.redial.success(addr)
		h.markPeersDirty()
	}
}

// addrConnected reports whether any active peer uses this remote dial address.
func (h *Host) addrConnected(addr string) bool {
	for _, p := range h.store.Active() {
		if p.RemoteAddr == addr {
			return true
		}
	}
	return false
}
