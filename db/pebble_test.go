package db

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestPebbleDB_PutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pdb, err := OpenPebble(filepath.Join(dir, "chaindata"))
	if err != nil {
		t.Fatal(err)
	}
	defer pdb.Close()

	key, val := []byte("hello"), []byte("world")
	if err := pdb.Put(key, val); err != nil {
		t.Fatal(err)
	}
	got, err := pdb.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("got %q want %q", got, val)
	}
}

func TestPebbleDB_HasDelete(t *testing.T) {
	pdb, err := OpenPebble(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer pdb.Close()

	key := []byte("k")
	ok, err := pdb.Has(key)
	if err != nil || ok {
		t.Fatalf("Has empty: ok=%v err=%v", ok, err)
	}
	_ = pdb.Put(key, []byte("v"))
	ok, _ = pdb.Has(key)
	if !ok {
		t.Fatal("expected has")
	}
	if err := pdb.Delete(key); err != nil {
		t.Fatal(err)
	}
	_, err = pdb.Get(key)
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPebbleDB_BatchAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	pdb, err := OpenPebble(path)
	if err != nil {
		t.Fatal(err)
	}

	b := pdb.NewBatch()
	_ = b.Put([]byte("a"), []byte("1"))
	_ = b.Put([]byte("b"), []byte("2"))
	if err := b.Write(); err != nil {
		t.Fatal(err)
	}
	if err := pdb.Close(); err != nil {
		t.Fatal(err)
	}

	pdb2, err := OpenPebble(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pdb2.Close()

	v, err := pdb2.Get([]byte("a"))
	if err != nil || string(v) != "1" {
		t.Fatalf("a = %q err=%v", v, err)
	}
	v, err = pdb2.Get([]byte("b"))
	if err != nil || string(v) != "2" {
		t.Fatalf("b = %q err=%v", v, err)
	}
}

func TestPebbleDB_IteratePrefix(t *testing.T) {
	pdb, err := OpenPebble(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer pdb.Close()

	_ = pdb.Put([]byte("a/1"), []byte("x"))
	_ = pdb.Put([]byte("a/2"), []byte("y"))
	_ = pdb.Put([]byte("b/1"), []byte("z"))

	var keys []string
	err = pdb.IteratePrefix([]byte("a/"), func(key, value []byte) error {
		keys = append(keys, string(key))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "a/1" || keys[1] != "a/2" {
		t.Fatalf("keys = %v", keys)
	}
}
