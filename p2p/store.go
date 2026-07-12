package p2p

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// KnownPeer is a remembered peer address for dial / PEX.
type KnownPeer struct {
	ID       PeerID
	Addr     string // host:port
	LastSeen time.Time
	BanScore int
}

// PeerStore tracks known and active peers.
type PeerStore struct {
	mu     sync.RWMutex
	known  map[PeerID]KnownPeer
	active map[PeerID]*Peer
	max    int
}

// NewPeerStore creates a store with a soft max active peers.
func NewPeerStore(maxActive int) *PeerStore {
	if maxActive <= 0 {
		maxActive = 25
	}
	return &PeerStore{
		known:  make(map[PeerID]KnownPeer),
		active: make(map[PeerID]*Peer),
		max:    maxActive,
	}
}

// Remember records or refreshes a known address (preserves ban score).
func (s *PeerStore) Remember(id PeerID, addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.known[id]
	k.ID = id
	if addr != "" {
		k.Addr = addr
	}
	k.LastSeen = time.Now()
	s.known[id] = k
}

// Forget removes a known peer entry.
func (s *PeerStore) Forget(id PeerID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.known, id)
}

// Known returns a snapshot of known peers.
func (s *PeerStore) Known() []KnownPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KnownPeer, 0, len(s.known))
	for _, k := range s.known {
		out = append(out, k)
	}
	return out
}

// KnownByID looks up a known peer.
func (s *PeerStore) KnownByID(id PeerID) (KnownPeer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.known[id]
	return k, ok
}

// AddActive registers a live peer. Returns error if at capacity or duplicate.
func (s *PeerStore) AddActive(p *Peer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.active[p.ID]; ok {
		return fmt.Errorf("p2p: peer %s already connected", p.ID.Hex())
	}
	if len(s.active) >= s.max {
		return fmt.Errorf("p2p: max peers reached (%d)", s.max)
	}
	s.active[p.ID] = p
	if p.RemoteAddr != "" {
		k := s.known[p.ID]
		k.ID = p.ID
		k.Addr = p.RemoteAddr
		k.LastSeen = time.Now()
		s.known[p.ID] = k
	}
	return nil
}

// RemoveActive drops a live peer.
func (s *PeerStore) RemoveActive(id PeerID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, id)
}

// Active returns a snapshot of connected peers.
func (s *PeerStore) Active() []*Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Peer, 0, len(s.active))
	for _, p := range s.active {
		out = append(out, p)
	}
	return out
}

// ActiveCount returns the number of connected peers.
func (s *PeerStore) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.active)
}

// GetActive returns a connected peer by id.
func (s *PeerStore) GetActive(id PeerID) (*Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.active[id]
	return p, ok
}

// FormatAddr builds host:port.
func FormatAddr(host string, port uint16) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}
