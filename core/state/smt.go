package state

import (
	"sort"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Sparse Merkle Tree depth (256-bit paths from Keccak256 of flat keys).
const smtDepth = 256

// Domain tags for node hashing (prevent leaf/internal confusion).
var (
	smtLeafTag     = []byte{0x00}
	smtInternalTag = []byte{0x01}
)

// emptyHashes[h] is the root of a completely empty subtree of height h
// (h=0 is an empty leaf placeholder; h=256 is the empty tree root).
var emptyHashes [smtDepth + 1]types.Hash

func init() {
	// Empty leaf placeholder (no key present).
	emptyHashes[0] = types.BytesToHash(crypto.Keccak256(smtLeafTag, []byte("empty")))
	for h := 0; h < smtDepth; h++ {
		emptyHashes[h+1] = hashInternal(emptyHashes[h], emptyHashes[h])
	}
}

// EmptySMTRoot is the StateRoot of an empty state (no accounts/storage/code).
func EmptySMTRoot() types.Hash {
	return emptyHashes[smtDepth]
}

func hashLeaf(value []byte) types.Hash {
	return types.BytesToHash(crypto.Keccak256(smtLeafTag, value))
}

func hashInternal(left, right types.Hash) types.Hash {
	return types.BytesToHash(crypto.Keccak256(smtInternalTag, left.Bytes(), right.Bytes()))
}

// smtPath is a 256-bit path (MSB of path[0] is the highest bit / root branch).
type smtPath [32]byte

// bit returns the i-th bit of the path from the root (i=0 is MSB).
func (p smtPath) bit(i int) byte {
	return (p[i/8] >> (7 - uint(i%8))) & 1
}

// ComputeSMTRoot builds a sparse Merkle root over flat state leaves.
//
// For each (key, value) leaf from the flat KV model:
//
//	path = Keccak256(key)
//	leafHash = Keccak256(0x00 || value)
//
// Internal nodes: Keccak256(0x01 || left || right). Missing subtrees use
// precomputed empty hashes. Same pre-state + same dirty leaves ⇒ same root.
func ComputeSMTRoot(leaves []leaf) types.Hash {
	if len(leaves) == 0 {
		return EmptySMTRoot()
	}
	// path → leaf hash (last write wins if duplicate paths — keys are unique)
	m := make(map[smtPath]types.Hash, len(leaves))
	for _, l := range leaves {
		var p smtPath
		copy(p[:], crypto.Keccak256(l.key))
		m[p] = hashLeaf(l.val)
	}
	return smtRootFromMap(m)
}

func smtRootFromMap(leaves map[smtPath]types.Hash) types.Hash {
	if len(leaves) == 0 {
		return EmptySMTRoot()
	}
	// Work list of (path, depth, hash) at current level; start at leaves (depth 0).
	type node struct {
		path smtPath
		hash types.Hash
	}
	level := make([]node, 0, len(leaves))
	for p, h := range leaves {
		level = append(level, node{path: p, hash: h})
	}
	// Sort by path for deterministic sibling pairing.
	sort.Slice(level, func(i, j int) bool {
		return bytesCompare(level[i].path[:], level[j].path[:]) < 0
	})

	// Climb from depth 0 (leaf) to depth 256 (root).
	// At height h, path bits 0..255-h identify the node; bit (255-h) is the child selector? 
	// We use: at step s (0..255), we combine nodes that share the first (255-s) bits.
	for height := 0; height < smtDepth; height++ {
		// height = distance from leaf. Parent sits at height+1.
		// Sibling bit is path.bit(smtDepth-1-height) — the lowest remaining free bit.
		bitIndex := smtDepth - 1 - height
		next := make([]node, 0, (len(level)+1)/2)
		for i := 0; i < len(level); {
			cur := level[i]
			// Parent path: clear bits below parent (keep bits 0..bitIndex-1)
			// Sibling is the other child at bitIndex.
			if i+1 < len(level) && samePrefix(level[i].path, level[i+1].path, bitIndex) {
				// Two children present
				left, right := level[i], level[i+1]
				if left.path.bit(bitIndex) == 1 {
					left, right = right, left
				}
				parentPath := left.path // bits above bitIndex match
				next = append(next, node{
					path: parentPath,
					hash: hashInternal(left.hash, right.hash),
				})
				i += 2
			} else {
				// Single child: pair with empty sibling
				var parentHash types.Hash
				if cur.path.bit(bitIndex) == 0 {
					parentHash = hashInternal(cur.hash, emptyHashes[height])
				} else {
					parentHash = hashInternal(emptyHashes[height], cur.hash)
				}
				next = append(next, node{path: cur.path, hash: parentHash})
				i++
			}
		}
		level = next
	}
	if len(level) != 1 {
		// Should not happen; fall back to folding
		h := EmptySMTRoot()
		for _, n := range level {
			h = hashInternal(h, n.hash)
		}
		return h
	}
	return level[0].hash
}

// samePrefix reports whether a and b agree on bits [0, bitIndex).
func samePrefix(a, b smtPath, bitIndex int) bool {
	for i := 0; i < bitIndex; i++ {
		if a.bit(i) != b.bit(i) {
			return false
		}
	}
	return true
}

func bytesCompare(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
