package faucet

import (
	"sync"
	"time"
)

// Limiter enforces a fixed-window count per key (IP or address).
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	entries map[string]*windowCount
}

type windowCount struct {
	start time.Time
	count int
}

// NewLimiter creates a rate limiter: at most `limit` events per `window` per key.
func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:   limit,
		window:  window,
		now:     time.Now,
		entries: make(map[string]*windowCount),
	}
}

// Allow records one event for key if under the limit. Returns false when limited.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.start) >= l.window {
		l.entries[key] = &windowCount{start: now, count: 1}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	return true
}

// Remaining returns how many events remain in the current window for key.
func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.start) >= l.window {
		return l.limit
	}
	left := l.limit - e.count
	if left < 0 {
		return 0
	}
	return left
}

// ResetForTest clears state (tests only).
func (l *Limiter) ResetForTest() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make(map[string]*windowCount)
}
