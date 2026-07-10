package state

import (
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Copy returns an independent StateDB sharing the same durable backend.
// Mutations on the copy do not affect the original until ApplyOverlay / Commit.
// Safe for optimistic parallel execution: each worker gets its own Copy of parent.
func (s *StateDB) Copy() *StateDB {
	out := &StateDB{
		db:            s.db,
		accounts:      make(map[crypto.Address]*types.Account, len(s.accounts)),
		storage:       make(map[storageID]types.Hash, len(s.storage)),
		code:          make(map[types.Hash][]byte, len(s.code)),
		accountDirty:  make(map[crypto.Address]struct{}),
		storageDirty:  make(map[storageID]struct{}),
		suicides:      make(map[crypto.Address]struct{}),
		journal:       newJournal(),
		accessList:    newAccessList(),
		transient:     newTransientStorage(),
		newContracts:  make(map[crypto.Address]struct{}),
		storageOrigin: make(map[storageID]types.Hash),
	}
	for addr, acc := range s.accounts {
		out.accounts[addr] = acc.Copy()
	}
	for id, v := range s.storage {
		out.storage[id] = v
	}
	for h, code := range s.code {
		out.code[h] = append([]byte(nil), code...)
	}
	// Parent dirty sets become warm cache on the copy (not dirty until re-written).
	// Speculative txs start from a clean journal relative to parent snapshot.
	return out
}

// ApplyOverlay merges dirty account/storage/code from src into s.
// Used by Dew-PE when a speculative execution is validated as non-conflicting.
func (s *StateDB) ApplyOverlay(src *StateDB) {
	if src == nil {
		return
	}
	for addr := range src.accountDirty {
		if _, dead := src.suicides[addr]; dead {
			s.suicides[addr] = struct{}{}
			delete(s.accounts, addr)
			s.accountDirty[addr] = struct{}{}
			continue
		}
		acc := src.accounts[addr]
		if acc == nil {
			continue
		}
		s.accounts[addr] = acc.Copy()
		s.accountDirty[addr] = struct{}{}
		if acc.CodeHash != types.EmptyCodeHash {
			if code, ok := src.code[acc.CodeHash]; ok {
				s.code[acc.CodeHash] = append([]byte(nil), code...)
			}
		}
	}
	for id := range src.storageDirty {
		if _, dead := src.suicides[id.addr]; dead {
			s.storage[id] = types.Hash{}
			s.storageDirty[id] = struct{}{}
			continue
		}
		s.storage[id] = src.storage[id]
		s.storageDirty[id] = struct{}{}
	}
	// Ensure account objects exist for storage-only dirties.
	for id := range src.storageDirty {
		if _, dead := src.suicides[id.addr]; dead {
			continue
		}
		if _, ok := s.accounts[id.addr]; !ok {
			if acc := src.accounts[id.addr]; acc != nil {
				s.accounts[id.addr] = acc.Copy()
				s.accountDirty[id.addr] = struct{}{}
			}
		}
	}
}

// SnapshotBalances returns a map of account balances for testing / diagnostics.
func (s *StateDB) SnapshotBalances(addrs []crypto.Address) map[crypto.Address]*uint256.Int {
	out := make(map[crypto.Address]*uint256.Int, len(addrs))
	for _, a := range addrs {
		out[a] = s.GetBalance(a)
	}
	return out
}
