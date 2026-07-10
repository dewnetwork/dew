package state

import (
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// CreateAccount creates a new account or resets an existing one (EVM CreateAccount).
func (s *StateDB) CreateAccount(addr crypto.Address) {
	prev := s.getAccount(addr)
	s.journal.append(createObjectChange{account: addr})
	acc := types.NewAccount()
	if prev != nil {
		// carry over balance on overwrite (CREATE collision handling is higher-level)
		acc.Balance = new(uint256.Int).Set(prev.GetBalance())
	}
	s.accounts[addr] = acc
	s.accountDirty[addr] = struct{}{}
}

// CreateContract marks addr as created in this transaction (EIP-6780).
func (s *StateDB) CreateContract(addr crypto.Address) {
	s.newContracts[addr] = struct{}{}
}

// SubBalance subtracts amount from the balance. Returns the previous balance.
func (s *StateDB) SubBalance(addr crypto.Address, amount *uint256.Int) uint256.Int {
	acc := s.GetOrNewAccount(addr)
	prev := new(uint256.Int).Set(acc.GetBalance())
	s.journal.append(balanceChange{account: addr, prev: prev})
	if amount != nil && !amount.IsZero() {
		acc.Balance = new(uint256.Int).Sub(prev, amount)
	}
	s.accountDirty[addr] = struct{}{}
	return *prev
}

// AddBalance journaled variant returning previous balance (EVM interface).
func (s *StateDB) AddBalancePrev(addr crypto.Address, amount *uint256.Int) uint256.Int {
	acc := s.GetOrNewAccount(addr)
	prev := new(uint256.Int).Set(acc.GetBalance())
	s.journal.append(balanceChange{account: addr, prev: prev})
	if amount != nil && !amount.IsZero() {
		acc.Balance = new(uint256.Int).Add(prev, amount)
	}
	s.accountDirty[addr] = struct{}{}
	return *prev
}

// SetBalanceJournaled sets balance with journal.
func (s *StateDB) SetBalanceJournaled(addr crypto.Address, bal *uint256.Int) {
	acc := s.GetOrNewAccount(addr)
	prev := new(uint256.Int).Set(acc.GetBalance())
	s.journal.append(balanceChange{account: addr, prev: prev})
	if bal == nil {
		acc.Balance = uint256.NewInt(0)
	} else {
		acc.Balance = new(uint256.Int).Set(bal)
	}
	s.accountDirty[addr] = struct{}{}
}

// SetNonceJournaled sets nonce with journal.
func (s *StateDB) SetNonceJournaled(addr crypto.Address, nonce uint64) {
	acc := s.GetOrNewAccount(addr)
	s.journal.append(nonceChange{account: addr, prev: acc.Nonce})
	acc.Nonce = nonce
	s.accountDirty[addr] = struct{}{}
}

// SetCodeJournaled sets code with journal.
func (s *StateDB) SetCodeJournaled(addr crypto.Address, code []byte) {
	acc := s.GetOrNewAccount(addr)
	prevHash := acc.CodeHash
	var prevCode []byte
	if prevHash != types.EmptyCodeHash {
		prevCode = s.GetCode(addr)
	}
	s.journal.append(codeChange{account: addr, prevHash: prevHash, prevCode: prevCode})
	if len(code) == 0 {
		acc.CodeHash = types.EmptyCodeHash
	} else {
		h := types.Keccak256Hash(code)
		acc.CodeHash = h
		s.code[h] = append([]byte(nil), code...)
	}
	s.accountDirty[addr] = struct{}{}
}

// SetStateJournaled sets storage and returns the previous value.
func (s *StateDB) SetStateJournaled(addr crypto.Address, key, value types.Hash) types.Hash {
	prev := s.GetState(addr, key)
	if prev == value {
		return prev
	}
	// record original for GetCommittedState if first write
	id := storageID{addr: addr, slot: key}
	if _, ok := s.storageOrigin[id]; !ok {
		s.storageOrigin[id] = prev
	}
	s.journal.append(storageChange{account: addr, key: key, prevalue: prev})
	_ = s.GetOrNewAccount(addr)
	s.storage[id] = value
	s.storageDirty[id] = struct{}{}
	return prev
}

// GetCommittedState returns the value at last commit (or origin before first dirty write).
func (s *StateDB) GetCommittedState(addr crypto.Address, key types.Hash) types.Hash {
	id := storageID{addr: addr, slot: key}
	if v, ok := s.storageOrigin[id]; ok {
		return v
	}
	// not dirtied — load from DB (same as GetState without using dirty)
	blob, err := s.db.Get(storageKey(addr, key))
	if err != nil {
		return types.Hash{}
	}
	return types.BytesToHash(blob)
}

// GetStorageRoot returns empty hash (flat model; no per-account storage trie yet).
func (s *StateDB) GetStorageRoot(addr crypto.Address) types.Hash {
	return types.Hash{}
}

// Empty reports EIP-161 emptiness.
func (s *StateDB) Empty(addr crypto.Address) bool {
	acc := s.getAccount(addr)
	if acc == nil {
		return true
	}
	return acc.Nonce == 0 && acc.GetBalance().IsZero() && acc.CodeHash == types.EmptyCodeHash
}

// ExistEVM reports existence including self-destructed accounts (vm.StateDB.Exist).
func (s *StateDB) ExistEVM(addr crypto.Address) bool {
	if _, ok := s.suicides[addr]; ok {
		return true
	}
	return s.Exist(addr)
}

// SelfDestruct marks account for deletion and clears balance; returns prior balance.
func (s *StateDB) SelfDestruct(addr crypto.Address) uint256.Int {
	acc := s.getAccount(addr)
	if acc == nil {
		return *uint256.NewInt(0)
	}
	prevBal := new(uint256.Int).Set(acc.GetBalance())
	_, prevSuicided := s.suicides[addr]
	s.journal.append(suicideChange{
		account:     addr,
		prev:        prevSuicided,
		prevBalance: prevBal,
	})
	s.suicides[addr] = struct{}{}
	acc.Balance = uint256.NewInt(0)
	s.accountDirty[addr] = struct{}{}
	return *prevBal
}

// HasSelfDestructed reports whether addr was marked self-destructed this tx.
func (s *StateDB) HasSelfDestructed(addr crypto.Address) bool {
	_, ok := s.suicides[addr]
	return ok
}

// SelfDestruct6780 implements EIP-6780 semantics.
func (s *StateDB) SelfDestruct6780(addr crypto.Address) (uint256.Int, bool) {
	if _, created := s.newContracts[addr]; created {
		return s.SelfDestruct(addr), true
	}
	// only send balance, do not destroy
	acc := s.getAccount(addr)
	if acc == nil {
		return *uint256.NewInt(0), false
	}
	prev := new(uint256.Int).Set(acc.GetBalance())
	s.journal.append(balanceChange{account: addr, prev: prev})
	acc.Balance = uint256.NewInt(0)
	s.accountDirty[addr] = struct{}{}
	return *prev, false
}

// AddRefund / SubRefund / GetRefund
func (s *StateDB) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

func (s *StateDB) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		panic("refund counter below zero")
	}
	s.refund -= gas
}

func (s *StateDB) GetRefund() uint64 { return s.refund }

// Snapshot / RevertToSnapshot
func (s *StateDB) Snapshot() int {
	id := s.nextRevisionID
	s.nextRevisionID++
	s.validRevisions = append(s.validRevisions, revision{id: id, journalIndex: s.journal.length()})
	return id
}

func (s *StateDB) RevertToSnapshot(revid int) {
	idx := -1
	for i := len(s.validRevisions) - 1; i >= 0; i-- {
		if s.validRevisions[i].id == revid {
			idx = i
			break
		}
	}
	if idx < 0 {
		panic("invalid snapshot id")
	}
	snapshot := s.validRevisions[idx].journalIndex
	s.journal.revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

// AddLog appends a log for the current transaction.
func (s *StateDB) AddLog(log *types.Log) {
	s.journal.append(addLogChange{})
	s.logs = append(s.logs, log)
}

// Logs returns logs accumulated since last clear.
func (s *StateDB) Logs() []*types.Log {
	return s.logs
}

// ClearLogs resets the log buffer (between transactions).
func (s *StateDB) ClearLogs() {
	s.logs = nil
}

// GetTransientState / SetTransientState (EIP-1153)
func (s *StateDB) GetTransientState(addr crypto.Address, key types.Hash) types.Hash {
	return s.transient.get(addr, key)
}

func (s *StateDB) SetTransientState(addr crypto.Address, key, value types.Hash) {
	prev := s.transient.get(addr, key)
	s.journal.append(transientStorageChange{account: addr, key: key, prevalue: prev})
	s.transient.set(addr, key, value)
}

// Access list helpers
func (s *StateDB) AddressInAccessList(addr crypto.Address) bool {
	return s.accessList.ContainsAddress(addr)
}

func (s *StateDB) SlotInAccessList(addr crypto.Address, slot types.Hash) (addressOk, slotOk bool) {
	return s.accessList.Contains(addr, slot)
}

func (s *StateDB) AddAddressToAccessList(addr crypto.Address) {
	if s.accessList.AddAddress(addr) {
		s.journal.append(accessListAddAccountChange{address: addr})
	}
}

func (s *StateDB) AddSlotToAccessList(addr crypto.Address, slot types.Hash) {
	addrMod, slotMod := s.accessList.AddSlot(addr, slot)
	if addrMod {
		s.journal.append(accessListAddAccountChange{address: addr})
	}
	if slotMod {
		s.journal.append(accessListAddSlotChange{address: addr, slot: slot})
	}
}

// Prepare sets up access list and resets transient storage for a new transaction (Berlin+).
func (s *StateDB) Prepare(
	sender, coinbase crypto.Address,
	dest *crypto.Address,
	precompiles []crypto.Address,
	// Berlin+ always warm sender/coinbase/dest/precompiles
) {
	s.accessList = newAccessList()
	s.transient = newTransientStorage()
	s.AddAddressToAccessList(sender)
	if dest != nil {
		s.AddAddressToAccessList(*dest)
	}
	for _, addr := range precompiles {
		s.AddAddressToAccessList(addr)
	}
	s.AddAddressToAccessList(coinbase)
	// clear per-tx bookkeeping that must not span txs
	s.newContracts = make(map[crypto.Address]struct{})
	s.refund = 0
	// keep logs; executor clears between txs
}

// Finalise ends a transaction: process suicides, clear journal revisions.
func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.suicides {
		delete(s.accounts, addr)
		s.accountDirty[addr] = struct{}{}
		// wipe storage cache for addr
		for id := range s.storage {
			if id.addr == addr {
				s.storage[id] = types.Hash{}
				s.storageDirty[id] = struct{}{}
			}
		}
	}
	if deleteEmptyObjects {
		for addr, acc := range s.accounts {
			if s.Empty(addr) && acc != nil {
				delete(s.accounts, addr)
				s.accountDirty[addr] = struct{}{}
			}
		}
	}
	// reset journal for next tx
	s.journal = newJournal()
	s.validRevisions = s.validRevisions[:0]
	s.suicides = make(map[crypto.Address]struct{})
	s.newContracts = make(map[crypto.Address]struct{})
	s.storageOrigin = make(map[storageID]types.Hash)
	s.refund = 0
	s.transient = newTransientStorage()
}

// GetCodeSize returns bytecode length.
func (s *StateDB) GetCodeSize(addr crypto.Address) int {
	return len(s.GetCode(addr))
}
