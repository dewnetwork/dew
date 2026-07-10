package state

import (
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// accessList tracks addresses and slots warm for EIP-2929 / EIP-2930.
type accessList struct {
	addresses map[crypto.Address]int
	slots     []map[types.Hash]struct{}
}

func newAccessList() *accessList {
	return &accessList{
		addresses: make(map[crypto.Address]int),
	}
}

func (a *accessList) ContainsAddress(addr crypto.Address) bool {
	_, ok := a.addresses[addr]
	return ok
}

func (a *accessList) Contains(addr crypto.Address, slot types.Hash) (addressOk, slotOk bool) {
	idx, ok := a.addresses[addr]
	if !ok {
		return false, false
	}
	if idx == -1 {
		return true, false
	}
	_, slotOk = a.slots[idx][slot]
	return true, slotOk
}

func (a *accessList) AddAddress(addr crypto.Address) bool {
	if _, ok := a.addresses[addr]; ok {
		return false
	}
	a.addresses[addr] = -1
	return true
}

func (a *accessList) AddSlot(addr crypto.Address, slot types.Hash) (addrChange, slotChange bool) {
	idx, addrPresent := a.addresses[addr]
	if !addrPresent || idx == -1 {
		// new address or address without slot map
		a.addresses[addr] = len(a.slots)
		slotmap := map[types.Hash]struct{}{slot: {}}
		a.slots = append(a.slots, slotmap)
		return !addrPresent, true
	}
	slotmap := a.slots[idx]
	if _, ok := slotmap[slot]; ok {
		return false, false
	}
	slotmap[slot] = struct{}{}
	return false, true
}

func (a *accessList) DeleteAddress(addr crypto.Address) {
	delete(a.addresses, addr)
}

func (a *accessList) DeleteSlot(addr crypto.Address, slot types.Hash) {
	idx, ok := a.addresses[addr]
	if !ok || idx == -1 {
		return
	}
	delete(a.slots[idx], slot)
	if len(a.slots[idx]) == 0 {
		a.addresses[addr] = -1
	}
}

// transientStorage is EIP-1153 per-tx storage.
type transientStorage map[crypto.Address]map[types.Hash]types.Hash

func newTransientStorage() transientStorage {
	return make(transientStorage)
}

func (t transientStorage) set(addr crypto.Address, key, value types.Hash) {
	if _, ok := t[addr]; !ok {
		t[addr] = make(map[types.Hash]types.Hash)
	}
	t[addr][key] = value
}

func (t transientStorage) get(addr crypto.Address, key types.Hash) types.Hash {
	if m, ok := t[addr]; ok {
		return m[key]
	}
	return types.Hash{}
}
