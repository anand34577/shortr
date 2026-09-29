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
)

func (l *ipLookup) Lookup(ip net.IP) GeoResult {
	if l == nil || l.settings == nil || ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return GeoResult{}
	}
	key := ip.String()
	l.mu.Lock()
	if e, ok := l.cache[key]; ok && time.Now().Before(e.exp) {
		l.mu.Unlock()
		return e.res
	}
	l.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), ipTimeout)
	defer cancel()
	enabled, base := l.settings(ctx)
	if !enabled || base == "" {
		return GeoResult{}
	}
	var res GeoResult
	ttl := ipCacheTTL
	if r, err := iploc.Lookup(ctx, base, key); err != nil {
		ttl = ipFailTTL
	} else {
		res = GeoResult{Country: r.CountryCode, Region: r.Subdivision, City: r.City}
	}
	l.mu.Lock()
	if l.cache == nil || len(l.cache) >= ipCacheSize {
		l.cache = make(map[string]ipEntry) // ponytail: wholesale reset, LRU if churn matters
	}
	l.cache[key] = ipEntry{res, time.Now().Add(ttl)}
	l.mu.Unlock()
	return res
}
