// Package wallet stores secp256k1 keys encrypted at rest (Web3 Secret Storage).
package wallet

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/keystore"

	"github.com/dewnetwork/dew/crypto"
)

// Store is a directory-backed encrypted keystore for dewcli wallets.
type Store struct {
	dir string
	ks  *keystore.KeyStore
}

// DefaultDir returns ~/.dew/keystore (or $DEW_KEYSTORE if set).
func DefaultDir() (string, error) {
	if env := os.Getenv("DEW_KEYSTORE"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("wallet: home dir: %w", err)
	}
	return filepath.Join(home, ".dew", "keystore"), nil
}

// Open opens or creates a keystore at dir.
func Open(dir string) (*Store, error) {
	if dir == "" {
		var err error
		dir, err = DefaultDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("wallet: mkdir %s: %w", dir, err)
	}
	// LightScryptN/P keep create snappy for CLI; production nodes can use Standard later.
	ks := keystore.NewKeyStore(dir, keystore.LightScryptN, keystore.LightScryptP)
	return &Store{dir: dir, ks: ks}, nil
}

// Dir returns the keystore directory path.
func (s *Store) Dir() string {
	return s.dir
}

// Create generates a new key, encrypts it with passphrase, and returns the address.
func (s *Store) Create(passphrase string) (crypto.Address, error) {
	acct, err := s.ks.NewAccount(passphrase)
	if err != nil {
		return crypto.Address{}, fmt.Errorf("wallet: create: %w", err)
	}
	var a crypto.Address
	copy(a[:], acct.Address[:])
	return a, nil
}

// ImportECDSA encrypts an existing private key into the store.
func (s *Store) ImportECDSA(key *ecdsa.PrivateKey, passphrase string) (crypto.Address, error) {
	acct, err := s.ks.ImportECDSA(key, passphrase)
	if err != nil {
		return crypto.Address{}, fmt.Errorf("wallet: import: %w", err)
	}
	var a crypto.Address
	copy(a[:], acct.Address[:])
	return a, nil
}

// List returns addresses currently in the keystore (sorted by hex).
func (s *Store) List() []crypto.Address {
	accts := s.ks.Accounts()
	out := make([]crypto.Address, 0, len(accts))
	for _, acct := range accts {
		var a crypto.Address
		copy(a[:], acct.Address[:])
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].HexNoChecksum() < out[j].HexNoChecksum()
	})
	return out
}

// Unlock loads the private key for address (caller should zeroize when done).
func (s *Store) Unlock(addr crypto.Address, passphrase string) (*ecdsa.PrivateKey, error) {
	for _, acct := range s.ks.Accounts() {
		var a crypto.Address
		copy(a[:], acct.Address[:])
		if a != addr {
			continue
		}
		keyJSON, err := os.ReadFile(acct.URL.Path)
		if err != nil {
			return nil, fmt.Errorf("wallet: read key: %w", err)
		}
		key, err := keystore.DecryptKey(keyJSON, passphrase)
		if err != nil {
			return nil, fmt.Errorf("wallet: decrypt: %w", err)
		}
		return key.PrivateKey, nil
	}
	return nil, fmt.Errorf("wallet: address %s not found", addr.Hex())
}
