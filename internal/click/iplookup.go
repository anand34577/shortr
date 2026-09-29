package click

import (
	"context"
	"net"
	"sync"
	"time"

	"shortr/internal/iploc"
)

// ipLookup resolves geo for an IP via the admin-configured lookup service
// (GET <base>/<ip>). Results are cached; failures back off so a dead service
// can't stall the click writer.
type ipLookup struct {
	settings func(context.Context) (enabled bool, baseURL string)
	mu       sync.Mutex
	cache    map[string]ipEntry
	// settings are re-read at most every ipSettingsTTL, not per click
	enabled   bool
	base      string
	settingsT time.Time
	// after a failure the whole service is skipped until this time
	downUntil time.Time
}

type ipEntry struct {
	res GeoResult
	exp time.Time
}

const (
	ipCacheTTL  = time.Hour
	ipFailTTL   = time.Minute
	ipCacheSize = 4096
	ipTimeout   = 2 * time.Second
	// ponytail: one failure pauses every lookup for ipDownTTL; a per-host breaker if this needs to be finer
	ipDownTTL     = 30 * time.Second
	ipSettingsTTL = 30 * time.Second
)

// available reports whether the service is configured and not currently backing off.
func (l *ipLookup) available() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if now := time.Now(); now.Sub(l.settingsT) > ipSettingsTTL && l.settings != nil {
		ctx, cancel := context.WithTimeout(context.Background(), ipTimeout)
		l.enabled, l.base = l.settings(ctx)
		cancel()
		l.settingsT = now
	}
	return l.enabled && l.base != "" && time.Now().After(l.downUntil)
}

func (l *ipLookup) Lookup(ip net.IP) GeoResult {
	if l == nil || l.settings == nil || ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return GeoResult{}
	}
	key := ip.String()
	now := time.Now()
	l.mu.Lock()
	if now.Sub(l.settingsT) > ipSettingsTTL {
		ctx, cancel := context.WithTimeout(context.Background(), ipTimeout)
		l.enabled, l.base = l.settings(ctx)
		cancel()
		l.settingsT = now
	}
	enabled, base, down := l.enabled, l.base, now.Before(l.downUntil)
	stale, cached := l.cache[key]
	if cached && now.Before(stale.exp) {
		l.mu.Unlock()
		return stale.res
	}
	l.mu.Unlock()
	if !enabled || base == "" || down {
		return stale.res // an expired answer beats none while the service is unavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), ipTimeout)
	defer cancel()
	var res GeoResult
	ttl := ipCacheTTL
	failed := false
	if r, err := iploc.Lookup(ctx, base, key); err != nil {
		res, ttl, failed = stale.res, ipFailTTL, true
	} else {
		res = GeoResult{Country: r.CountryCode, Region: r.Subdivision, City: r.City}
	}
	l.mu.Lock()
	if failed {
		l.downUntil = time.Now().Add(ipDownTTL)
	}
	if l.cache == nil || len(l.cache) >= ipCacheSize {
		l.cache = make(map[string]ipEntry) // ponytail: wholesale reset, LRU if churn matters
	}
	l.cache[key] = ipEntry{res, time.Now().Add(ttl)}
	l.mu.Unlock()
	return res
}
