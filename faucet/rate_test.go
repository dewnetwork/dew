package faucet

import (
	"testing"
	"time"
)

func TestLimiterAllowsUpToLimit(t *testing.T) {
	l := NewLimiter(2, time.Hour)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return base }

	if !l.Allow("0xabc") {
		t.Fatal("first allow")
	}
	if !l.Allow("0xabc") {
		t.Fatal("second allow")
	}
	if l.Allow("0xabc") {
		t.Fatal("third should be limited")
	}
	if rem := l.Remaining("0xabc"); rem != 0 {
		t.Fatalf("remaining=%d want 0", rem)
	}
}

func TestLimiterResetsAfterWindow(t *testing.T) {
	l := NewLimiter(1, time.Hour)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	l.now = func() time.Time { return now }

	if !l.Allow("ip1") {
		t.Fatal("first")
	}
	if l.Allow("ip1") {
		t.Fatal("same window blocked")
	}
	now = base.Add(time.Hour)
	if !l.Allow("ip1") {
		t.Fatal("after window should allow")
	}
}

func TestLimiterIndependentKeys(t *testing.T) {
	l := NewLimiter(1, time.Hour)
	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("independent keys")
	}
	if l.Allow("a") {
		t.Fatal("a limited")
	}
}
