// Package state implements the flat account/storage model for Dew.
//
// Hot path is pure KV (no MPT walks). StateRoot is an SMT commitment over the
// flat snapshot at commit time (Phase C3; see docs/protocol/state.md).
package state

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

// StateDB is a caching layer over a flat Database.
type StateDB struct {
	db db.Database

	accounts map[crypto.Address]*types.Account
	storage  map[storageID]types.Hash
	code     map[types.Hash][]byte

	// existence / dirty tracking
	accountDirty map[crypto.Address]struct{}
	storageDirty map[storageID]struct{}
	suicides     map[crypto.Address]struct{}

	// EVM execution support
	journal        *journal
	validRevisions []revision
	nextRevisionID int
	refund         uint64
	logs           []*types.Log
	accessList     *accessList
	transient      transientStorage
	newContracts   map[crypto.Address]struct{}
	storageOrigin  map[storageID]types.Hash // committed value before first dirty write in tx

	// Dew-PE access tracking (optional; not concurrent-safe with other writers)
	trackAccess bool
	accessSet   *AccessSet
}

type storageID struct {
	addr crypto.Address
	slot types.Hash
}

// New creates a StateDB backed by db.
func New(database db.Database) *StateDB {
	return &StateDB{
		db:            database,
		accounts:      make(map[crypto.Address]*types.Account),
		storage:       make(map[storageID]types.Hash),
		code:          make(map[types.Hash][]byte),
		accountDirty:  make(map[crypto.Address]struct{}),
		storageDirty:  make(map[storageID]struct{}),
		suicides:      make(map[crypto.Address]struct{}),
		journal:       newJournal(),
		accessList:    newAccessList(),
		transient:     newTransientStorage(),
		newContracts:  make(map[crypto.Address]struct{}),
		storageOrigin: make(map[storageID]types.Hash),
	}
}

// Exist reports whether the account is present (including empty accounts created in-cache).
func (s *StateDB) Exist(addr crypto.Address) bool {
	s.noteRead(AccountKey(addr))
	if _, ok := s.suicides[addr]; ok {
		return false
	}
	if _, ok := s.accounts[addr]; ok {
		return true
	}
	ok, err := s.db.Has(accountKey(addr))
	return err == nil && ok
}

// GetOrNewAccount returns the account, creating an empty EOA in cache if missing.
func (s *StateDB) GetOrNewAccount(addr crypto.Address) *types.Account {
	if acc := s.getAccount(addr); acc != nil {
		return acc
	}
	acc := types.NewAccount()
	s.accounts[addr] = acc
	s.accountDirty[addr] = struct{}{}
	return acc
}

func (s *StateDB) getAccount(addr crypto.Address) *types.Account {
	if _, dead := s.suicides[addr]; dead {
		return nil
	}
	if acc, ok := s.accounts[addr]; ok {
		return acc
	}
	blob, err := s.db.Get(accountKey(addr))
	if err != nil {
		return nil
	}
	acc, err := decodeAccount(blob)
	if err != nil {
		return nil
	}
	s.accounts[addr] = acc
	return acc
}

// GetBalance returns the account balance (zero if missing).
func (s *StateDB) GetBalance(addr crypto.Address) *uint256.Int {
	s.noteRead(AccountKey(addr))
	acc := s.getAccount(addr)
	if acc == nil {
		return uint256.NewInt(0)
	}
	return new(uint256.Int).Set(acc.GetBalance())
}

// SetBalance sets the balance, creating the account if needed.
func (s *StateDB) SetBalance(addr crypto.Address, bal *uint256.Int) {
	s.noteWrite(AccountKey(addr))
	acc := s.GetOrNewAccount(addr)
	if bal == nil {
		acc.Balance = uint256.NewInt(0)
	} else {
		acc.Balance = new(uint256.Int).Set(bal)
	}
	s.accountDirty[addr] = struct{}{}
}

// AddBalance adds amount to the balance (unjournaled helper for genesis/tests).
// Prefer AddBalancePrev during EVM execution so Snapshot/Revert works.
func (s *StateDB) AddBalance(addr crypto.Address, amount *uint256.Int) {
	s.AddBalancePrev(addr, amount)
}

// SetBalance sets the balance, creating the account if needed (unjournaled for genesis).
// During EVM execution use SetBalanceJournaled.
func (s *StateDB) SetBalanceRaw(addr crypto.Address, bal *uint256.Int) {
	acc := s.GetOrNewAccount(addr)
	if bal == nil {
		acc.Balance = uint256.NewInt(0)
	} else {
		acc.Balance = new(uint256.Int).Set(bal)
	}
	s.accountDirty[addr] = struct{}{}
}

// GetNonce returns the account nonce (0 if missing).
func (s *StateDB) GetNonce(addr crypto.Address) uint64 {
	s.noteRead(AccountKey(addr))
	acc := s.getAccount(addr)
	if acc == nil {
		return 0
	}
	return acc.Nonce
}

// SetNonce sets the nonce.
func (s *StateDB) SetNonce(addr crypto.Address, nonce uint64) {
	s.noteWrite(AccountKey(addr))
	acc := s.GetOrNewAccount(addr)
	acc.Nonce = nonce
	s.accountDirty[addr] = struct{}{}
}

// GetCodeHash returns the code hash (EmptyCodeHash if missing).
func (s *StateDB) GetCodeHash(addr crypto.Address) types.Hash {
	s.noteRead(AccountKey(addr))
	acc := s.getAccount(addr)
	if acc == nil {
		return types.EmptyCodeHash
	}
	return acc.CodeHash
}

// GetCode returns contract bytecode.
func (s *StateDB) GetCode(addr crypto.Address) []byte {
	s.noteRead(AccountKey(addr))
	hash := s.GetCodeHash(addr)
	if hash == types.EmptyCodeHash {
		return nil
	}
	if code, ok := s.code[hash]; ok {
		return append([]byte(nil), code...)
	}
	blob, err := s.db.Get(codeKey(hash))
	if err != nil {
		return nil
	}
	s.code[hash] = blob
	return append([]byte(nil), blob...)
}

// SetCode sets contract code and updates CodeHash.
func (s *StateDB) SetCode(addr crypto.Address, code []byte) {
	s.noteWrite(AccountKey(addr))
	acc := s.GetOrNewAccount(addr)
	if len(code) == 0 {
		acc.CodeHash = types.EmptyCodeHash
		s.accountDirty[addr] = struct{}{}
		return
	}
	h := types.Keccak256Hash(code)
	acc.CodeHash = h
	s.code[h] = append([]byte(nil), code...)
	s.accountDirty[addr] = struct{}{}
}

// GetState returns a storage slot value.
func (s *StateDB) GetState(addr crypto.Address, slot types.Hash) types.Hash {
	s.noteRead(StorageKeyAccess(addr, slot))
	id := storageID{addr: addr, slot: slot}
	if v, ok := s.storage[id]; ok {
		return v
	}
	blob, err := s.db.Get(storageKey(addr, slot))
	if err != nil {
		return types.Hash{}
	}
	v := types.BytesToHash(blob)
	s.storage[id] = v
	return v
}

// SetState sets a storage slot.
func (s *StateDB) SetState(addr crypto.Address, slot, value types.Hash) {
	s.noteWrite(StorageKeyAccess(addr, slot))
	_ = s.GetOrNewAccount(addr) // ensure account exists
	id := storageID{addr: addr, slot: slot}
	s.storage[id] = value
	s.storageDirty[id] = struct{}{}
}

// Commit flushes dirty objects to the database and returns a deterministic StateRoot.
func (s *StateDB) Commit() (types.Hash, error) {
	if err := s.FlushDirtyTo(s.db); err != nil {
		return types.Hash{}, err
	}
	root, err := s.IntermediateRoot()
	if err != nil {
		return types.Hash{}, err
	}
	s.ClearDirty()
	return root, nil
}

// FlushDirtyTo writes dirty accounts/storage/code into w without clearing dirty sets.
// Use with a db.Batch so chain keys can share one atomic Write; call ClearDirty after Write succeeds.
func (s *StateDB) FlushDirtyTo(w stateWriter) error {
	if w == nil {
		return fmt.Errorf("state: nil writer")
	}
	for addr := range s.accountDirty {
		if _, dead := s.suicides[addr]; dead {
			if err := w.Delete(accountKey(addr)); err != nil {
				return err
			}
			continue
		}
		acc := s.accounts[addr]
		if acc == nil {
			continue
		}
		blob, err := encodeAccount(acc)
		if err != nil {
			return err
		}
		if err := w.Put(accountKey(addr), blob); err != nil {
			return err
		}
		if acc.CodeHash != types.EmptyCodeHash {
			if code, ok := s.code[acc.CodeHash]; ok {
				if err := w.Put(codeKey(acc.CodeHash), code); err != nil {
					return err
				}
			}
		}
	}
	for id := range s.storageDirty {
		val := s.storage[id]
		if val.IsZero() {
			if err := w.Delete(storageKey(id.addr, id.slot)); err != nil {
				return err
			}
			continue
		}
		if err := w.Put(storageKey(id.addr, id.slot), val.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

// ClearDirty drops dirty tracking after a successful durable flush (keeps account cache warm).
func (s *StateDB) ClearDirty() {
	s.accountDirty = make(map[crypto.Address]struct{})
	s.storageDirty = make(map[storageID]struct{})
	s.suicides = make(map[crypto.Address]struct{})
}

// stateWriter is satisfied by db.Database and db.Batch.
type stateWriter interface {
	Put(key, value []byte) error
	Delete(key []byte) error
}

// IntermediateRoot computes the SMT commitment over flat state (Phase C3).
//
// Hot path remains flat KV. At commit / intermediate root:
//
//	collect leaves: "a"+addr → account RLP, "s"+addr+slot → 32-byte value,
//	                "c"+codeHash → bytecode
//	root = ComputeSMTRoot(leaves)  // path = Keccak256(key), sparse binary tree
//
// Migration: chains that used the Phase A provisional sorted-leaf root must
// re-genesis / wipe (dev-only). Wire meaning of header.StateRoot freezes at C6.
func (s *StateDB) IntermediateRoot() (types.Hash, error) {
	leaves, err := s.collectLeaves()
	if err != nil {
		return types.Hash{}, err
	}
	return ComputeSMTRoot(leaves), nil
}

type leaf struct {
	key []byte
	val []byte
}

func (s *StateDB) collectLeaves() ([]leaf, error) {
	// Start from durable snapshot when the backend can iterate.
	merged := make(map[string][]byte)
	if it, ok := s.db.(db.IteratePrefix); ok {
		err := it.IteratePrefix(nil, func(key, value []byte) error {
			// Only state prefixes
			if len(key) == 0 {
				return nil
			}
			switch key[0] {
			case prefixAccount, prefixStorage, prefixCode:
				merged[string(key)] = append([]byte(nil), value...)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Overlay account cache (authoritative for loaded/dirty entries).
	for addr, acc := range s.accounts {
		k := string(accountKey(addr))
		if _, dead := s.suicides[addr]; dead {
			delete(merged, k)
			continue
		}
		blob, err := encodeAccount(acc)
		if err != nil {
			return nil, err
		}
		merged[k] = blob
		if acc.CodeHash != types.EmptyCodeHash {
			code := s.code[acc.CodeHash]
			if code == nil {
				// try db-backed via GetCode path without forcing account create
				if blob, err := s.db.Get(codeKey(acc.CodeHash)); err == nil {
					code = blob
					s.code[acc.CodeHash] = blob
				}
			}
			if code != nil {
				merged[string(codeKey(acc.CodeHash))] = append([]byte(nil), code...)
			}
		}
	}
	for id, val := range s.storage {
		k := string(storageKey(id.addr, id.slot))
		if _, dead := s.suicides[id.addr]; dead || val.IsZero() {
			delete(merged, k)
			continue
		}
		merged[k] = val.Bytes()
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]leaf, 0, len(keys))
	for _, k := range keys {
		out = append(out, leaf{key: []byte(k), val: merged[k]})
	}
	return out, nil
}

func encodeAccount(acc *types.Account) ([]byte, error) {
	bal := acc.GetBalance().ToBig()
	return rlp.EncodeToBytes([]interface{}{
		acc.Nonce,
		bal,
		acc.CodeHash.Bytes(),
	})
}

func decodeAccount(blob []byte) (*types.Account, error) {
	var raw struct {
		Nonce    uint64
		Balance  *big.Int
		CodeHash []byte
	}
	if err := rlp.DecodeBytes(blob, &raw); err != nil {
		return nil, fmt.Errorf("state: decode account: %w", err)
	}
	acc := types.NewAccount()
	acc.Nonce = raw.Nonce
	acc.SetBalanceBig(raw.Balance)
	acc.CodeHash = types.BytesToHash(raw.CodeHash)
	return acc, nil
}
