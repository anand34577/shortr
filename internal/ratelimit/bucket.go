// Package ratelimit implements an in-memory token-bucket limiter keyed by an
// arbitrary string (IP, user id, API key id, email). per-node only
// — a shared limiter is only needed once shortr runs multiple replicas,
// which v1 doesn't support anyway.
package ratelimit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Rate parses strings like "200/10s" or "120/60s".
type Rate struct {
	N        int
	Interval time.Duration
}

func ParseRate(s string) (Rate, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return Rate{}, fmt.Errorf("invalid rate %q: expected N/duration", s)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 0 {
		return Rate{}, fmt.Errorf("invalid rate %q: bad count", s)
	}
	d, err := time.ParseDuration(parts[1])
	if err != nil || d <= 0 {
		return Rate{}, fmt.Errorf("invalid rate %q: bad duration", s)
	}
	return Rate{N: n, Interval: d}, nil
}

type bucketState struct {
	tokens     float64
	lastRefill time.Time
	lastAccess time.Time
}

type Limiter struct {
	rate    Rate
	mu      sync.Mutex
	buckets map[string]*bucketState
	maxKeys int
}

// New creates a limiter. rate.N == 0 disables limiting (Allow always true).
func New(rate Rate) *Limiter {
	return &Limiter{rate: rate, buckets: make(map[string]*bucketState), maxKeys: 500000}
}

// Allow reports whether one request for key is permitted right now, and
// consumes a token if so.
func (l *Limiter) Allow(key string) bool {
	if l.rate.N == 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.maxKeys {
			l.evictOldestLocked()
		}
		b = &bucketState{tokens: float64(l.rate.N), lastRefill: now}
		l.buckets[key] = b
	}
	b.lastAccess = now

	elapsed := now.Sub(b.lastRefill).Seconds()
	refillRate := float64(l.rate.N) / l.rate.Interval.Seconds()
	b.tokens += elapsed * refillRate
	if b.tokens > float64(l.rate.N) {
		b.tokens = float64(l.rate.N)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Peek reports whether a request for key would currently be permitted,
// without consuming a token. Pair it with Allow to rate-limit only failures
// (e.g. login attempts) and Reset to forgive on success.
func (l *Limiter) Peek(key string) bool {
	if l.rate.N == 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		return true
	}
	tokens := b.tokens + time.Since(b.lastRefill).Seconds()*float64(l.rate.N)/l.rate.Interval.Seconds()
	return tokens >= 1
}

// Reset forgets key, restoring its full allowance.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.buckets, key)
	l.mu.Unlock()
}

// evictOldestLocked makes room when the map is full (e.g. under an
// IP-scanning attack). Buckets that have already refilled to capacity carry no
// state (dropping them is indistinguishable from never having seen the key),
// so those go first; only if the sample has none do we drop the
// least-recently-used entries. Sampling keeps this O(1000). Caller holds l.mu.
func (l *Limiter) evictOldestLocked() {
	type kv struct {
		key string
		t   time.Time
	}
	now := time.Now()
	refill := float64(l.rate.N) / l.rate.Interval.Seconds()
	sample := make([]kv, 0, 1000)
	removed := 0
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.lastRefill).Seconds()*refill >= float64(l.rate.N) {
			delete(l.buckets, k)
			removed++
		} else {
			sample = append(sample, kv{k, b.lastAccess})
		}
		if len(sample)+removed >= 1000 {
			break
		}
	}
	if removed > 0 {
		return
	}
	sort.Slice(sample, func(i, j int) bool { return sample[i].t.Before(sample[j].t) })
	for i := 0; i < len(sample)/10+1 && i < len(sample); i++ {
		delete(l.buckets, sample[i].key)
	}
}

// GC removes buckets untouched for longer than idle — run periodically from
// a background ticker to bound memory.
func (l *Limiter) GC(idle time.Duration) {
	cutoff := time.Now().Add(-idle)
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if !b.lastAccess.After(cutoff) {
			delete(l.buckets, k)
		}
	}
}

func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
