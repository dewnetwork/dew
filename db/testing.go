package db

import (
	"path/filepath"
	"testing"
)

// OpenTest opens a Pebble database under t.TempDir() and registers Close on cleanup.
// Use this in unit tests instead of any in-memory backend.
func OpenTest(t testing.TB) *PebbleDB {
	t.Helper()
	pdb, err := OpenPebble(filepath.Join(t.TempDir(), "pebble"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pdb.Close() })
	return pdb
}
