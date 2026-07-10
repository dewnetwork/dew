package state

import (
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// journal records state mutations for Snapshot / RevertToSnapshot.
type journal struct {
	entries []journalEntry
	dirties map[crypto.Address]int // dirty count (for Finalise bookkeeping)
}

type journalEntry interface {
	revert(s *StateDB)
}

type revision struct {
	id           int
	journalIndex int
}

func newJournal() *journal {
	return &journal{
		dirties: make(map[crypto.Address]int),
	}
}

func (j *journal) append(entry journalEntry) {
	j.entries = append(j.entries, entry)
}

func (j *journal) revert(s *StateDB, snapshot int) {
	for i := len(j.entries) - 1; i >= snapshot; i-- {
		j.entries[i].revert(s)
	}
	j.entries = j.entries[:snapshot]
}

func (j *journal) length() int { return len(j.entries) }

// --- journal entries ---

type balanceChange struct {
	account crypto.Address
	prev    *uint256.Int
}

func (ch balanceChange) revert(s *StateDB) {
	acc := s.GetOrNewAccount(ch.account)
	acc.Balance = new(uint256.Int).Set(ch.prev)
	s.accountDirty[ch.account] = struct{}{}
}

type nonceChange struct {
	account crypto.Address
	prev    uint64
}

func (ch nonceChange) revert(s *StateDB) {
	acc := s.GetOrNewAccount(ch.account)
	acc.Nonce = ch.prev
	s.accountDirty[ch.account] = struct{}{}
}

type codeChange struct {
	account  crypto.Address
	prevHash types.Hash
	prevCode []byte
}

func (ch codeChange) revert(s *StateDB) {
	acc := s.GetOrNewAccount(ch.account)
	acc.CodeHash = ch.prevHash
	if ch.prevHash != types.EmptyCodeHash && ch.prevCode != nil {
		s.code[ch.prevHash] = append([]byte(nil), ch.prevCode...)
	}
	s.accountDirty[ch.account] = struct{}{}
}

type storageChange struct {
	account       crypto.Address
	key, prevalue types.Hash
}

func (ch storageChange) revert(s *StateDB) {
	id := storageID{addr: ch.account, slot: ch.key}
	s.storage[id] = ch.prevalue
	s.storageDirty[id] = struct{}{}
}

type createObjectChange struct {
	account crypto.Address
}

func (ch createObjectChange) revert(s *StateDB) {
	delete(s.accounts, ch.account)
	delete(s.accountDirty, ch.account)
	delete(s.newContracts, ch.account)
}

type suicideChange struct {
	account     crypto.Address
	prev        bool
	prevBalance *uint256.Int
}

func (ch suicideChange) revert(s *StateDB) {
	if !ch.prev {
		delete(s.suicides, ch.account)
	} else {
		s.suicides[ch.account] = struct{}{}
	}
	if acc := s.getAccount(ch.account); acc != nil && ch.prevBalance != nil {
		acc.Balance = new(uint256.Int).Set(ch.prevBalance)
		s.accountDirty[ch.account] = struct{}{}
	}
}

type refundChange struct {
	prev uint64
}

func (ch refundChange) revert(s *StateDB) {
	s.refund = ch.prev
}

type addLogChange struct{}

func (ch addLogChange) revert(s *StateDB) {
	if n := len(s.logs); n > 0 {
		s.logs = s.logs[:n-1]
	}
}

type accessListAddAccountChange struct {
	address crypto.Address
}

func (ch accessListAddAccountChange) revert(s *StateDB) {
	s.accessList.DeleteAddress(ch.address)
}

type accessListAddSlotChange struct {
	address crypto.Address
	slot    types.Hash
}

func (ch accessListAddSlotChange) revert(s *StateDB) {
	s.accessList.DeleteSlot(ch.address, ch.slot)
}

type transientStorageChange struct {
	account       crypto.Address
	key, prevalue types.Hash
}

func (ch transientStorageChange) revert(s *StateDB) {
	s.transient.set(ch.account, ch.key, ch.prevalue)
}

type touchChange struct {
	account crypto.Address
}

func (ch touchChange) revert(s *StateDB) {
	// no-op: touch is for dirtiness; account remains
}
