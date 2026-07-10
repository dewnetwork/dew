package types

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dewnetwork/dew/crypto"
)

// Hash is a 32-byte Keccak digest (block hash, roots, code hash, …).
type Hash [32]byte

// Bytes returns a copy of the hash.
func (h Hash) Bytes() []byte {
	b := make([]byte, 32)
	copy(b, h[:])
	return b
}

// Hex returns 0x-prefixed lowercase hex.
func (h Hash) Hex() string {
	return "0x" + hex.EncodeToString(h[:])
}

// String implements fmt.Stringer.
func (h Hash) String() string { return h.Hex() }

// IsZero reports whether h is the zero hash.
func (h Hash) IsZero() bool { return h == Hash{} }

// BytesToHash copies b into a Hash (right-aligned if shorter; truncated if longer).
func BytesToHash(b []byte) Hash {
	var h Hash
	if len(b) > 32 {
		b = b[len(b)-32:]
	}
	copy(h[32-len(b):], b)
	return h
}

// HexToHash parses a 0x-prefixed or bare hex hash.
func HexToHash(s string) (Hash, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) != 64 {
		return Hash{}, fmt.Errorf("types: hash must be 32 bytes (64 hex chars), got len %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Hash{}, fmt.Errorf("types: invalid hash hex: %w", err)
	}
	return BytesToHash(b), nil
}

// Keccak256Hash returns Keccak-256(data...) as Hash.
func Keccak256Hash(data ...[]byte) Hash {
	return BytesToHash(crypto.Keccak256(data...))
}

// EmptyCodeHash is Keccak-256 of empty bytecode (EOA code hash).
var EmptyCodeHash = Keccak256Hash(nil)
