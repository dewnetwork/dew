// Package crypto provides Ethereum-compatible secp256k1 keys, Keccak-256 hashing,
// address derivation, and ECDSA sign/verify for Dew.
//
// Primitives match Ethereum so MetaMask / Foundry keys interoperate unchanged.
// Implementation delegates to audited go-ethereum crypto packages.
package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// Address is a 20-byte Ethereum-compatible account identifier.
type Address [20]byte

// Bytes returns a copy of the address bytes.
func (a Address) Bytes() []byte {
	b := make([]byte, 20)
	copy(b, a[:])
	return b
}

// String implements fmt.Stringer as EIP-55 checksum hex.
func (a Address) String() string {
	return a.Hex()
}

// GenerateKey creates a new random secp256k1 private key (CSPRNG).
func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ethcrypto.GenerateKey()
}

// FromECDSA exports the private key as 32 bytes.
func FromECDSA(key *ecdsa.PrivateKey) []byte {
	return ethcrypto.FromECDSA(key)
}

// ToECDSA parses a 32-byte private key.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	return ethcrypto.ToECDSA(d)
}

// FromECDSAPub exports the uncompressed public key (65 bytes: 0x04 || X || Y).
func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	return ethcrypto.FromECDSAPub(pub)
}

// PubkeyToAddress derives an EOA address:
// Keccak-256(uncompressedPubKey[1:])[12:32].
func PubkeyToAddress(pub *ecdsa.PublicKey) Address {
	ea := ethcrypto.PubkeyToAddress(*pub)
	var a Address
	copy(a[:], ea[:])
	return a
}

// HexToAddress parses a 0x-prefixed or bare 40-hex-char address.
func HexToAddress(s string) (Address, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) != 40 {
		return Address{}, fmt.Errorf("crypto: address must be 20 bytes (40 hex chars), got len %d", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, fmt.Errorf("crypto: invalid address hex: %w", err)
	}
	var a Address
	copy(a[:], b)
	return a, nil
}

// Keccak256 computes the Ethereum Keccak-256 hash (not FIPS SHA3-256).
func Keccak256(data ...[]byte) []byte {
	return ethcrypto.Keccak256(data...)
}

// Sign signs a 32-byte digest with the private key.
// Returns a 65-byte signature [R || S || V] with V in {0, 1}.
func Sign(digest []byte, key *ecdsa.PrivateKey) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("crypto: digest must be 32 bytes, got %d", len(digest))
	}
	return ethcrypto.Sign(digest, key)
}

// Ecrecover recovers the uncompressed public key from a 65-byte signature over digest.
func Ecrecover(digest, sig []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("crypto: digest must be 32 bytes, got %d", len(digest))
	}
	return ethcrypto.Ecrecover(digest, sig)
}

// VerifySignature checks that sig (64-byte R||S) is a valid signature of digest by pub.
// pub may be compressed (33) or uncompressed (65) encoding.
func VerifySignature(pub, digest, sig []byte) bool {
	return ethcrypto.VerifySignature(pub, digest, sig)
}
