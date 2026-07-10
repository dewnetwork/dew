package state

import (
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// AccessKey identifies an account or storage slot for Dew-PE conflict detection.
type AccessKey string

// AccountKey returns the access key for any field of an account (balance, nonce, code).
func AccountKey(addr crypto.Address) AccessKey {
	return AccessKey("a" + string(addr[:]))
}

// StorageKeyAccess returns the access key for a storage slot.
func StorageKeyAccess(addr crypto.Address, slot types.Hash) AccessKey {
	return AccessKey("s" + string(addr[:]) + string(slot[:]))
}

// AccessSet records keys read and written during one transaction execution.
type AccessSet struct {
	Reads  map[AccessKey]struct{}
	Writes map[AccessKey]struct{}
}

// NewAccessSet creates an empty access set.
func NewAccessSet() *AccessSet {
	return &AccessSet{
		Reads:  make(map[AccessKey]struct{}),
		Writes: make(map[AccessKey]struct{}),
	}
}

// RecordRead marks a key as read (no-op if already written by this tx).
func (a *AccessSet) RecordRead(k AccessKey) {
	if a == nil {
		return
	}
	if _, w := a.Writes[k]; w {
		return
	}
	a.Reads[k] = struct{}{}
}

// RecordWrite marks a key as written and removes it from the pure-read set.
func (a *AccessSet) RecordWrite(k AccessKey) {
	if a == nil {
		return
	}
	delete(a.Reads, k)
	a.Writes[k] = struct{}{}
}

// ConflictsWith reports whether this set's reads or writes touch any key in earlier writes.
func (a *AccessSet) ConflictsWith(earlierWrites map[AccessKey]struct{}) bool {
	if a == nil {
		return false
	}
	for k := range a.Reads {
		if _, ok := earlierWrites[k]; ok {
			return true
		}
	}
	for k := range a.Writes {
		if _, ok := earlierWrites[k]; ok {
			return true
		}
	}
	return false
}

// MergeWrites copies write keys into dst.
func (a *AccessSet) MergeWrites(dst map[AccessKey]struct{}) {
	if a == nil {
		return
	}
	for k := range a.Writes {
		dst[k] = struct{}{}
	}
}

// StartAccessTracking enables read/write set recording for Dew-PE.
func (s *StateDB) StartAccessTracking() {
	s.trackAccess = true
	s.accessSet = NewAccessSet()
}

// TakeAccessSet returns the recorded set and disables tracking.
func (s *StateDB) TakeAccessSet() *AccessSet {
	s.trackAccess = false
	out := s.accessSet
	s.accessSet = nil
	return out
}

// AccessSet returns the current set without clearing (may be nil).
func (s *StateDB) AccessSet() *AccessSet {
	return s.accessSet
}

func (s *StateDB) noteRead(k AccessKey) {
	if s.trackAccess && s.accessSet != nil {
		s.accessSet.RecordRead(k)
	}
}

func (s *StateDB) noteWrite(k AccessKey) {
	if s.trackAccess && s.accessSet != nil {
		s.accessSet.RecordWrite(k)
	}
}
