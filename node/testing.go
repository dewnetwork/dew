package node

import (
	"path/filepath"
	"testing"

	"github.com/dewnetwork/dew/config"
)

// OpenTest opens a fresh Pebble-backed node under t.TempDir()/chaindata.
// Registers Close on cleanup. Preferred constructor for tests.
func OpenTest(t testing.TB, g *config.Genesis) *Node {
	t.Helper()
	n, err := Open(g, filepath.Join(t.TempDir(), "chaindata"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return n
}
