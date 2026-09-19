// Package ratelimit implements an in-memory token-bucket limiter keyed by an
// arbitrary string (IP, user id, API key id, email). ponytail: per-node only
// — a shared limiter is only needed once shortr runs multiple replicas,
// which v1 doesn't support anyway (see PLAN.md §25).
package ratelimit

import (
	"fmt"
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

// evictOldestLocked drops a handful of the least-recently-used buckets when
// the map grows unbounded (e.g. under an IP-scanning attack). Caller holds
// l.mu.
func (l *Limiter) evictOldestLocked() {
	type kv struct {
		key string
		t   time.Time
	}
	victims := make([]kv, 0, 100)
	for k, b := range l.buckets {
		victims = append(victims, kv{k, b.lastAccess})
		if len(victims) >= 1000 {
			break
		}
	}
	// simple partial selection: remove the oldest ~10% sampled
	for i := 0; i < len(victims)/10+1 && i < len(victims); i++ {
		oldestIdx := i
		for j := i + 1; j < len(victims); j++ {
			if victims[j].t.Before(victims[oldestIdx].t) {
				oldestIdx = j
			}
		}
		delete(l.buckets, victims[oldestIdx].key)
		victims[oldestIdx] = victims[i]
	}
}

// GC removes buckets untouched for longer than idle — run periodically from
// a background ticker to bound memory (PLAN.md §19.2).
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
