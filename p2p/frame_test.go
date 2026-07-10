package p2p

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("hello-dew")
	if err := WriteFrame(&buf, MsgPing, payload, 1024); err != nil {
		t.Fatal(err)
	}
	f, err := ReadFrame(&buf, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if f.Type != MsgPing {
		t.Fatalf("type %d", f.Type)
	}
	if string(f.Payload) != string(payload) {
		t.Fatalf("payload %q", f.Payload)
	}
}

func TestFrameRejectsOversized(t *testing.T) {
	var buf bytes.Buffer
	big := make([]byte, 100)
	if err := WriteFrame(&buf, MsgPing, big, 50); err == nil {
		t.Fatal("expected oversized write error")
	}
}
