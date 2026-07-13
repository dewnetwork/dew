package rpc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestParseBlockNumber_Tags(t *testing.T) {
	cases := []struct {
		in   interface{}
		want BlockNumberTag
	}{
		{nil, LatestBlock},
		{"latest", LatestBlock},
		{"", LatestBlock},
		{"earliest", EarliestBlock},
		{"pending", PendingBlock},
		{"0x0", 0},
		{"0x10", 16},
		{"42", 42},
		{float64(7), 7},
		{json.Number("99"), 99},
	}
	for _, tc := range cases {
		got, err := ParseBlockNumber(tc.in)
		if err != nil {
			t.Fatalf("ParseBlockNumber(%v): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseBlockNumber(%v)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseBlockNumber_RejectsOverflow(t *testing.T) {
	// MaxUint64 hex — cannot fit in int64 without wrap.
	overflowHex := "0x" + strings.Repeat("f", 16) // 2^64-1
	if _, err := ParseBlockNumber(overflowHex); err == nil {
		t.Fatal("expected error for uint64 > MaxInt64")
	}
	// MaxInt64+1
	justOver := EncodeUint64(uint64(math.MaxInt64) + 1)
	if _, err := ParseBlockNumber(justOver); err == nil {
		t.Fatal("expected error for MaxInt64+1")
	}
	// MaxInt64 itself must succeed.
	maxOK := EncodeUint64(uint64(math.MaxInt64))
	got, err := ParseBlockNumber(maxOK)
	if err != nil {
		t.Fatalf("MaxInt64 should parse: %v", err)
	}
	if got != BlockNumberTag(math.MaxInt64) {
		t.Fatalf("got %d want MaxInt64", got)
	}
}

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
