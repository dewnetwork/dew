package db

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
)

// PebbleDB is a durable Database backed by CockroachDB Pebble.
type PebbleDB struct {
	mu     sync.RWMutex
	db     *pebble.DB
	closed bool
}

// OpenPebble opens (or creates) a Pebble database at path.
func OpenPebble(path string) (*PebbleDB, error) {
	if path == "" {
		return nil, fmt.Errorf("db: empty pebble path")
	}
	pdb, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("db: open pebble %s: %w", path, err)
	}
	return &PebbleDB{db: pdb}, nil
}

func (p *PebbleDB) Has(key []byte) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return false, errClosed
	}
	_, closer, err := p.db.Get(key)
	if err == pebble.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = closer.Close()
	return true, nil
}

func (p *PebbleDB) Get(key []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil, errClosed
	}
	val, closer, err := p.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), val...)
	_ = closer.Close()
	return out, nil
}

func (p *PebbleDB) Put(key []byte, value []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return errClosed
	}
	return p.db.Set(key, value, pebble.Sync)
}

func (p *PebbleDB) Delete(key []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return errClosed
	}
	return p.db.Delete(key, pebble.Sync)
}

func (p *PebbleDB) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	err := p.db.Close()
	p.db = nil
	return err
}

// IteratePrefix implements IteratePrefix (ordered by key).
func (p *PebbleDB) IteratePrefix(prefix []byte, fn func(key, value []byte) error) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return errClosed
	}
	iter, err := p.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return err
	}
	defer func() { _ = iter.Close() }()

	for iter.First(); iter.Valid(); iter.Next() {
		k := append([]byte(nil), iter.Key()...)
		v := append([]byte(nil), iter.Value()...)
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return iter.Error()
}

// NewBatch returns a batch that writes into this PebbleDB on Write.
func (p *PebbleDB) NewBatch() Batch {
	return &pebbleBatch{db: p, batch: p.db.NewBatch()}
}

type pebbleBatch struct {
	db    *PebbleDB
	batch *pebble.Batch
}

func (b *pebbleBatch) Put(key, value []byte) error {
	return b.batch.Set(key, value, nil)
}

func (b *pebbleBatch) Delete(key []byte) error {
	return b.batch.Delete(key, nil)
}

func (b *pebbleBatch) Write() error {
	b.db.mu.RLock()
	defer b.db.mu.RUnlock()
	if b.db.closed {
		return errClosed
	}
	return b.batch.Commit(pebble.Sync)
}

func (b *pebbleBatch) Reset() {
	b.batch.Reset()
}

// prefixUpperBound returns the exclusive upper bound for keys with the given prefix.
// Empty prefix → nil (full range).
func prefixUpperBound(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	// Increment the last byte that is not 0xff; truncate after it.
	up := append([]byte(nil), prefix...)
	for i := len(up) - 1; i >= 0; i-- {
		if up[i] != 0xff {
			up[i]++
			return up[:i+1]
		}
	}
	// prefix is all 0xff — no finite upper bound; scan with prefix check only.
	return nil
}
