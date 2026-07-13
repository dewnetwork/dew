package rpc

import (
	"strings"
	"testing"
)

func TestDecodeHash_LeftPadsShortHex(t *testing.T) {
	cases := []struct {
		in   string
		want string // full 0x + 64 hex
	}{
		{"0x0", "0x0000000000000000000000000000000000000000000000000000000000000000"},
		{"0x1", "0x0000000000000000000000000000000000000000000000000000000000000001"},
		{"0x1a", "0x000000000000000000000000000000000000000000000000000000000000001a"},
		{"0x0000000000000000000000000000000000000000000000000000000000000002", "0x0000000000000000000000000000000000000000000000000000000000000002"},
	}
	for _, tc := range cases {
		h, err := DecodeHash(tc.in)
		if err != nil {
			t.Fatalf("DecodeHash(%q): %v", tc.in, err)
		}
		if got := EncodeHash(h); got != tc.want {
			t.Fatalf("DecodeHash(%q)=%s want %s", tc.in, got, tc.want)
		}
	}
}

func TestDecodeHash_RejectsTooLong(t *testing.T) {
	long := "0x" + "11" + strings.Repeat("00", 32)
	_, err := DecodeHash(long)
	if err == nil {
		t.Fatal("expected error for overlong hash")
	}
}
