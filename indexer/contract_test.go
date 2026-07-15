package indexer

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStore_ContractUpsert(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	addr := "0xAbcDef0123456789AbcDef0123456789AbcDef01"
	abi := `[{"type":"function","name":"foo","inputs":[],"outputs":[]}]`
	if err := s.UpsertContract(ContractRecord{
		Address:  addr,
		Name:     "Foo",
		ABIJSON:  abi,
		Source:   "contract Foo {}",
		Compiler: "solc 0.8.24",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetContract(addr)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Name != "Foo" || got.ABIJSON != abi {
		t.Fatalf("%+v", got)
	}
	if got.Address != normalizeAddr(addr) {
		t.Fatalf("addr = %s", got.Address)
	}
	created := got.CreatedAt
	time.Sleep(10 * time.Millisecond)
	if err := s.UpsertContract(ContractRecord{
		Address: addr,
		ABIJSON: `[]`,
		Name:    "Bar",
	}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.GetContract(addr)
	if got.Name != "Bar" || got.ABIJSON != `[]` {
		t.Fatalf("upsert: %+v", got)
	}
	if got.CreatedAt != created {
		t.Fatalf("created_at changed: %d -> %d", created, got.CreatedAt)
	}
	if got.UpdatedAt < created {
		t.Fatalf("updated_at %d < created %d", got.UpdatedAt, created)
	}
}

func TestHTTP_ContractRegisterAndGet(t *testing.T) {
	dir := t.TempDir()
	ix, err := New(Config{
		RPCURL:     "http://127.0.0.1:1", // unused for contract routes
		SQLitePath: filepath.Join(dir, "ix.db"),
		ListenAddr: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })

	srv, err := NewServer(ix, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Start() }()
	// wait for listen
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Addr() != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	base := srv.ListenURL()
	addr := "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

	// GET missing
	res, err := http.Get(base + "/v1/contract/" + addr)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("get missing status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	// POST invalid abi object
	bad, _ := json.Marshal(map[string]interface{}{"abi": map[string]string{"x": "y"}})
	res, err = http.Post(base+"/v1/contract/"+addr, "application/json", bytes.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad abi status %d body %s", res.StatusCode, body)
	}

	// POST ok
	payload, _ := json.Marshal(map[string]interface{}{
		"name":     "TestToken",
		"abi":      []map[string]interface{}{{"type": "function", "name": "name", "inputs": []interface{}{}, "outputs": []interface{}{}}},
		"source":   "contract TestToken {}",
		"compiler": "solc 0.8.24",
	})
	res, err = http.Post(base+"/v1/contract/"+addr, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("post status %d body %s", res.StatusCode, body)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["status"] != "registered" || got["name"] != "TestToken" {
		t.Fatalf("post body %s", body)
	}

	// GET ok
	res, err = http.Get(base + "/v1/contract/" + addr)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get status %d body %s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), `"status":"registered"`) && !strings.Contains(string(body), `"status": "registered"`) {
		// encoder may space
		var m map[string]interface{}
		_ = json.Unmarshal(body, &m)
		if m["status"] != "registered" {
			t.Fatalf("get body %s", body)
		}
	}
}

func TestHTTP_ContractTokenAuth(t *testing.T) {
	dir := t.TempDir()
	ix, err := New(Config{
		RPCURL:      "http://127.0.0.1:1",
		SQLitePath:  filepath.Join(dir, "ix.db"),
		ListenAddr:  "127.0.0.1:0",
		VerifyToken: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	srv, err := NewServer(ix, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	go func() { _ = srv.Start() }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Addr() != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	base := srv.ListenURL()
	addr := "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	payload := []byte(`{"abi":[]}`)

	res, err := http.Post(base+"/v1/contract/"+addr, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", res.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodPost, base+"/v1/contract/"+addr, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dew-Verify-Token", "secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200 with token, got %d %s", res.StatusCode, body)
	}
}
