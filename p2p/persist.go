package p2p

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dewnetwork/dew/crypto"
)

// peersFileVersion is the on-disk schema version for peers.json.
const peersFileVersion = 1

// BanDialThreshold: known peers at or above this ban score are not dialed.
const BanDialThreshold = 100

// DefaultPeerTTL drops idle known peers after this duration (bootnodes exempt).
const DefaultPeerTTL = 7 * 24 * time.Hour

type peersFile struct {
	Version int              `json:"version"`
	Peers   []knownPeerJSON  `json:"peers"`
}

type knownPeerJSON struct {
	ID       string `json:"id"`
	Addr     string `json:"addr"`
	LastSeen string `json:"lastSeen"`
	BanScore int    `json:"banScore"`
}

// LoadFromFile merges peers from path into the store. Missing file is OK.
// Corrupt / unreadable file returns an error (caller may start empty).
func (s *PeerStore) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("p2p: read peer store: %w", err)
	}
	var f peersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("p2p: parse peer store: %w", err)
	}
	if f.Version != 0 && f.Version != peersFileVersion {
		return fmt.Errorf("p2p: unsupported peer store version %d", f.Version)
	}
	now := time.Now()
	for _, e := range f.Peers {
		id, err := crypto.HexToAddress(e.ID)
		if err != nil {
			continue
		}
		if e.Addr == "" {
			continue
		}
		lastSeen := now
		if e.LastSeen != "" {
			if t, err := time.Parse(time.RFC3339, e.LastSeen); err == nil {
				lastSeen = t
			}
		}
		s.mu.Lock()
		s.known[id] = KnownPeer{
			ID:       id,
			Addr:     e.Addr,
			LastSeen: lastSeen,
			BanScore: e.BanScore,
		}
		s.mu.Unlock()
	}
	return nil
}

// SaveToFile writes known peers atomically (temp file + rename).
func (s *PeerStore) SaveToFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("p2p: peer store dir: %w", err)
	}
	known := s.Known()
	f := peersFile{Version: peersFileVersion, Peers: make([]knownPeerJSON, 0, len(known))}
	for _, k := range known {
		f.Peers = append(f.Peers, knownPeerJSON{
			ID:       k.ID.Hex(),
			Addr:     k.Addr,
			LastSeen: k.LastSeen.UTC().Format(time.RFC3339),
			BanScore: k.BanScore,
		})
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("p2p: write peer store: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("p2p: rename peer store: %w", err)
	}
	return nil
}

// EvictOlderThan removes known peers with LastSeen older than maxAge.
// keepAddrs are never removed (bootnode addresses). Active peers are not
// removed from known solely by eviction of their ID if still active — we only
// touch the known map; active sessions stay until disconnect.
func (s *PeerStore) EvictOlderThan(maxAge time.Duration, keepAddrs map[string]struct{}) int {
	if maxAge <= 0 {
		maxAge = DefaultPeerTTL
	}
	cutoff := time.Now().Add(-maxAge)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, k := range s.known {
		if keepAddrs != nil {
			if _, ok := keepAddrs[k.Addr]; ok {
				continue
			}
		}
		if _, active := s.active[id]; active {
			continue
		}
		if k.LastSeen.Before(cutoff) {
			delete(s.known, id)
			n++
		}
	}
	return n
}

// AddBanScore increases ban score for a known peer (creates entry if needed).
func (s *PeerStore) AddBanScore(id PeerID, delta int) {
	if delta == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.known[id]
	if !ok {
		k = KnownPeer{ID: id, LastSeen: time.Now()}
	}
	k.BanScore += delta
	if k.BanScore < 0 {
		k.BanScore = 0
	}
	k.LastSeen = time.Now()
	s.known[id] = k
}

// Dialable returns known peers that should be considered for outbound dial.
func (s *PeerStore) Dialable() []KnownPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KnownPeer, 0, len(s.known))
	for _, k := range s.known {
		if k.Addr == "" {
			continue
		}
		if k.BanScore >= BanDialThreshold {
			continue
		}
		if _, ok := s.active[k.ID]; ok {
			continue
		}
		out = append(out, k)
	}
	return out
}


