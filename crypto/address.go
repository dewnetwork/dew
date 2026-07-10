package crypto

import (
	"encoding/hex"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// Hex returns the EIP-55 checksum hex encoding with 0x prefix.
func (a Address) Hex() string {
	return ethcommon.BytesToAddress(a[:]).Hex()
}

// MustHexToAddress panics on invalid input (for tests/fixtures only).
func MustHexToAddress(s string) Address {
	a, err := HexToAddress(s)
	if err != nil {
		panic(err)
	}
	return a
}

// HexNoChecksum returns lowercase 0x-prefixed hex without EIP-55 mixed case.
func (a Address) HexNoChecksum() string {
	return "0x" + hex.EncodeToString(a[:])
}

// Equal reports whether a and b are the same address.
func (a Address) Equal(b Address) bool {
	return a == b
}

// IsZero reports whether the address is the zero address.
func (a Address) IsZero() bool {
	return a == Address{}
}

// NormalizeAddressHex lowercases a hex address for comparison (keeps 0x).
func NormalizeAddressHex(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		s = "0x" + s
	}
	return strings.ToLower(s)
}
