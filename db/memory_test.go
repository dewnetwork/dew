package db

import (
	"bytes"
	"errors"
	"testing"
)

func TestMemoryDB_PutGetRoundTrip(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()

	key := []byte("account/0xabc")
	val := []byte(`{"nonce":1}`)

	if err := mdb.Put(key, val); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := mdb.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("Get = %q, want %q", got, val)
	}

	// Mutation of returned slice must not affect store.
	got[0] = 'X'
	got2, _ := mdb.Get(key)
	if got2[0] == 'X' {
		t.Fatal("store mutated via Get return value")
	}
}

func TestMemoryDB_HasDelete(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()

	key := []byte("k")
	ok, err := mdb.Has(key)
	if err != nil || ok {
		t.Fatalf("Has empty = %v, %v", ok, err)
	}

	_ = mdb.Put(key, []byte("v"))
	ok, _ = mdb.Has(key)
	if !ok {
		t.Fatal("Has after Put = false")
	}

	if err := mdb.Delete(key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = mdb.Get(key)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
}

func TestMemoryDB_Overwrite(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()

	_ = mdb.Put([]byte("k"), []byte("v1"))
	_ = mdb.Put([]byte("k"), []byte("v2"))
	got, _ := mdb.Get([]byte("k"))
	if !bytes.Equal(got, []byte("v2")) {
		t.Fatalf("got %q, want v2", got)
	}
}

func TestMemoryDB_Batch(t *testing.T) {
	mdb := NewMemoryDB()
	defer mdb.Close()

	b := mdb.NewBatch()
	_ = b.Put([]byte("a"), []byte("1"))
	_ = b.Put([]byte("b"), []byte("2"))
	if mdb.Len() != 0 {
		t.Fatal("batch should not write before Write()")
	}
	if err := b.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if mdb.Len() != 2 {
		t.Fatalf("Len = %d, want 2", mdb.Len())
	}
}
