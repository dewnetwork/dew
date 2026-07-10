// Package db defines a minimal key-value store used by state and chain data.
package db

import "errors"

// ErrNotFound is returned when a key is missing.
var ErrNotFound = errors.New("db: key not found")

// Database is a simple byte-keyed store.
// Implementations must be safe for concurrent use if documented as such;
// MemoryDB is safe for concurrent use.
type Database interface {
	// Has reports whether key exists.
	Has(key []byte) (bool, error)
	// Get returns a copy of the value for key.
	Get(key []byte) ([]byte, error)
	// Put stores value under key (overwrites if present).
	Put(key []byte, value []byte) error
	// Delete removes key. Missing keys are not an error.
	Delete(key []byte) error
	// Close releases resources.
	Close() error
}

// Batch groups writes for atomic flush (optional optimization; MemoryDB applies immediately).
type Batch interface {
	Put(key, value []byte) error
	Delete(key []byte) error
	Write() error
	Reset()
}

// Batcher is a Database that can create write batches.
type Batcher interface {
	Database
	NewBatch() Batch
}

// IteratePrefix walks keys with the given prefix (nil/empty = all keys).
// Keys and values must not be retained after the callback returns unless copied.
type IteratePrefix interface {
	IteratePrefix(prefix []byte, fn func(key, value []byte) error) error
}
