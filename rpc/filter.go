package rpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// HTTP filter poll limits (abuse control; independent of WebSocket limits).
const (
	MaxFilters    = 128
	FilterIdleTTL = 5 * time.Minute
)

type filterKind int

const (
	filterLogs filterKind = iota
	filterBlocks
	filterPending
)

type installedFilter struct {
	id         string
	kind       filterKind
	addrs      []crypto.Address
	topics     [][]types.Hash
	fromTag    interface{} // optional raw block tag for eth_getFilterLogs
	toTag      interface{}
	lastPolled uint64
	lastAccess time.Time
}

// snapshot is a copy of filter fields used outside the store lock.
type filterSnapshot struct {
	id         string
	kind       filterKind
	addrs      []crypto.Address
	topics     [][]types.Hash
	fromTag    interface{}
	toTag      interface{}
	lastPolled uint64
}

type filterStore struct {
	mu    sync.Mutex
	byID  map[string]*installedFilter
	nowFn func() time.Time
}

func newFilterStore() *filterStore {
	return &filterStore{
		byID:  make(map[string]*installedFilter),
		nowFn: time.Now,
	}
}

func (s *filterStore) now() time.Time {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now()
}

func (s *filterStore) purgeExpiredLocked() {
	cutoff := s.now().Add(-FilterIdleTTL)
	for id, f := range s.byID {
		if f.lastAccess.Before(cutoff) {
			delete(s.byID, id)
		}
	}
}

func newFilterID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return EncodeBytes(b[:])
}

func (s *filterStore) install(f *installedFilter) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	if len(s.byID) >= MaxFilters {
		return "", fmt.Errorf("too many filters (max %d)", MaxFilters)
	}
	if f.id == "" {
		f.id = newFilterID()
	}
	f.lastAccess = s.now()
	s.byID[f.id] = f
	return f.id, nil
}

func (s *filterStore) snapshot(id string) (filterSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	f, ok := s.byID[id]
	if !ok {
		return filterSnapshot{}, fmt.Errorf("filter not found")
	}
	f.lastAccess = s.now()
	addrs := make([]crypto.Address, len(f.addrs))
	copy(addrs, f.addrs)
	topics := make([][]types.Hash, len(f.topics))
	for i, level := range f.topics {
		if level == nil {
			topics[i] = nil
			continue
		}
		topics[i] = append([]types.Hash(nil), level...)
	}
	return filterSnapshot{
		id:         f.id,
		kind:       f.kind,
		addrs:      addrs,
		topics:     topics,
		fromTag:    f.fromTag,
		toTag:      f.toTag,
		lastPolled: f.lastPolled,
	}, nil
}

func (s *filterStore) advance(id string, tip uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.byID[id]; ok {
		f.lastPolled = tip
		f.lastAccess = s.now()
	}
}

func (s *filterStore) uninstall(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	if _, ok := s.byID[id]; !ok {
		return false
	}
	delete(s.byID, id)
	return true
}

func (a *API) ethNewFilter(params json.RawMessage) (interface{}, error) {
	var p []map[string]interface{}
	if len(params) > 0 && string(params) != "null" && string(params) != "[]" {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params")
		}
	}
	tip := a.n.BlockNumber()
	f := &installedFilter{kind: filterLogs, lastPolled: tip}
	if len(p) > 0 {
		q := p[0]
		addrs, topics, err := parseLogFilterObject(q)
		if err != nil {
			return nil, err
		}
		f.addrs = addrs
		f.topics = topics
		if v, ok := q["fromBlock"]; ok {
			f.fromTag = v
		}
		if v, ok := q["toBlock"]; ok {
			f.toTag = v
		}
	}
	return a.filters.install(f)
}

func (a *API) ethNewBlockFilter(_ json.RawMessage) (interface{}, error) {
	tip := a.n.BlockNumber()
	return a.filters.install(&installedFilter{kind: filterBlocks, lastPolled: tip})
}

func (a *API) ethNewPendingTransactionFilter(_ json.RawMessage) (interface{}, error) {
	tip := a.n.BlockNumber()
	return a.filters.install(&installedFilter{kind: filterPending, lastPolled: tip})
}

func (a *API) ethUninstallFilter(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	return a.filters.uninstall(p[0]), nil
}

func (a *API) ethGetFilterChanges(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	snap, err := a.filters.snapshot(p[0])
	if err != nil {
		return nil, err
	}

	tip := a.n.BlockNumber()
	from := snap.lastPolled + 1

	var result interface{}
	switch snap.kind {
	case filterPending:
		result = []interface{}{}
	case filterBlocks:
		if from > tip {
			result = []string{}
		} else {
			hashes := make([]string, 0, tip-from+1)
			for h := from; h <= tip; h++ {
				b := a.n.GetBlockByNumber(h)
				if b == nil {
					continue
				}
				hashes = append(hashes, EncodeHash(b.Hash()))
			}
			result = hashes
		}
	case filterLogs:
		if from > tip {
			result = []map[string]interface{}{}
		} else {
			matched := a.n.FilterLogs(from, tip, snap.addrs, snap.topics)
			out := make([]map[string]interface{}, 0, len(matched))
			for _, il := range matched {
				out = append(out, formatIndexedLog(il))
			}
			result = out
		}
	default:
		return nil, fmt.Errorf("filter not found")
	}

	// Always advance cursor to tip after a successful poll (including empty pending).
	a.filters.advance(snap.id, tip)
	return result, nil
}

func (a *API) ethGetFilterLogs(params json.RawMessage) (interface{}, error) {
	var p []string
	if err := json.Unmarshal(params, &p); err != nil || len(p) < 1 {
		return nil, fmt.Errorf("invalid params")
	}
	snap, err := a.filters.snapshot(p[0])
	if err != nil {
		return nil, err
	}
	if snap.kind != filterLogs {
		return nil, fmt.Errorf("not a log filter")
	}

	head := a.n.BlockNumber()
	from, to := uint64(0), head
	if snap.fromTag != nil {
		tag, err := ParseBlockNumber(snap.fromTag)
		if err != nil {
			return nil, err
		}
		from, err = ResolveBlockNumber(tag, head)
		if err != nil {
			return nil, err
		}
	}
	if snap.toTag != nil {
		tag, err := ParseBlockNumber(snap.toTag)
		if err != nil {
			return nil, err
		}
		to, err = ResolveBlockNumber(tag, head)
		if err != nil {
			return nil, err
		}
	}
	matched := a.n.FilterLogs(from, to, snap.addrs, snap.topics)
	out := make([]map[string]interface{}, 0, len(matched))
	for _, il := range matched {
		out = append(out, formatIndexedLog(il))
	}
	return out, nil
}

// parseLogFilterObject extracts address/topics from an eth_getLogs-style object.
func parseLogFilterObject(q map[string]interface{}) ([]crypto.Address, [][]types.Hash, error) {
	var addrs []crypto.Address
	if v, ok := q["address"]; ok {
		switch t := v.(type) {
		case string:
			a, err := DecodeAddress(t)
			if err != nil {
				return nil, nil, err
			}
			addrs = append(addrs, a)
		case []interface{}:
			for _, x := range t {
				a, err := DecodeAddress(fmt.Sprint(x))
				if err != nil {
					return nil, nil, err
				}
				addrs = append(addrs, a)
			}
		}
	}
	var topics [][]types.Hash
	if v, ok := q["topics"].([]interface{}); ok {
		for _, level := range v {
			if level == nil {
				topics = append(topics, nil)
				continue
			}
			switch t := level.(type) {
			case string:
				h, err := DecodeHash(t)
				if err != nil {
					return nil, nil, err
				}
				topics = append(topics, []types.Hash{h})
			case []interface{}:
				var alts []types.Hash
				for _, x := range t {
					if x == nil {
						continue
					}
					h, err := DecodeHash(fmt.Sprint(x))
					if err != nil {
						return nil, nil, err
					}
					alts = append(alts, h)
				}
				topics = append(topics, alts)
			}
		}
	}
	return addrs, topics, nil
}
