package state

import (
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Flat KV key layout (docs/protocol/state.md):
//
//	a + address(20)              → account RLP
//	s + address(20) + slot(32)   → storage value (32 bytes)
//	c + codeHash(32)             → bytecode
//	meta/stateRoot               → last committed root (optional)
const (
	prefixAccount byte = 'a'
	prefixStorage byte = 's'
	prefixCode    byte = 'c'
)

func accountKey(addr crypto.Address) []byte {
	k := make([]byte, 1+20)
	k[0] = prefixAccount
	copy(k[1:], addr[:])
	return k
}

func storageKey(addr crypto.Address, slot types.Hash) []byte {
	k := make([]byte, 1+20+32)
	k[0] = prefixStorage
	copy(k[1:21], addr[:])
	copy(k[21:], slot[:])
	return k
}

func codeKey(codeHash types.Hash) []byte {
	k := make([]byte, 1+32)
	k[0] = prefixCode
	copy(k[1:], codeHash[:])
	return k
}
