package db

import (
	"sync"
)

// MemoryDB is an in-memory Database, suitable for tests and early phases.
type MemoryDB struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryDB creates an empty in-memory store.
func NewMemoryDB() *MemoryDB {
	return &MemoryDB{data: make(map[string][]byte)}
}

func (m *MemoryDB) Has(key []byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[string(key)]
	return ok, nil
}

func (m *MemoryDB) Get(key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[string(key)]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (m *MemoryDB) Put(key []byte, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[string(key)] = cp
	return nil
}

func (m *MemoryDB) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, string(key))
	return nil
}

func (m *MemoryDB) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = nil
	return nil
}

// Len returns the number of keys (test/debug helper).
func (m *MemoryDB) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// IteratePrefix implements IteratePrefix.
func (m *MemoryDB) IteratePrefix(prefix []byte, fn func(key, value []byte) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pre := string(prefix)
	for k, v := range m.data {
		if pre != "" && (len(k) < len(pre) || k[:len(pre)] != pre) {
			continue
		}
		if err := fn([]byte(k), v); err != nil {
			return err
		}
	}
	return nil
}

// NewBatch returns a batch that writes into this MemoryDB on Write.
func (m *MemoryDB) NewBatch() Batch {
	return &memoryBatch{db: m}
}

type memoryBatch struct {
	db   *MemoryDB
	ops  []batchOp
	size int
}

type batchOp struct {
	del   bool
	key   []byte
	value []byte
}

func (b *memoryBatch) Put(key, value []byte) error {
	k := append([]byte(nil), key...)
	v := append([]byte(nil), value...)
	b.ops = append(b.ops, batchOp{key: k, value: v})
	b.size += len(k) + len(v)
	return nil
}

func (b *memoryBatch) Delete(key []byte) error {
	k := append([]byte(nil), key...)
	b.ops = append(b.ops, batchOp{del: true, key: k})
	b.size += len(k)
	return nil
}

func (b *memoryBatch) Write() error {
	b.db.mu.Lock()
	defer b.db.mu.Unlock()
	if b.db.data == nil {
		return errorsClosed()
	}
	for _, op := range b.ops {
		if op.del {
			delete(b.db.data, string(op.key))
			continue
		}
		b.db.data[string(op.key)] = op.value
	}
	return nil
}

func (b *memoryBatch) Reset() {
	b.ops = b.ops[:0]
	b.size = 0
}

func errorsClosed() error {
	return errClosed
}

var errClosed = errString("db: closed")

type errString string

func (e errString) Error() string { return string(e) }
