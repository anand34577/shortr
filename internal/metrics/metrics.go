// Package metrics is a hand-rolled Prometheus text-exposition counter set —
// about a dozen series, not worth pulling in client_golang's dependency tree
// for (PLAN.md §3.1).
package metrics

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type counterKey struct {
	name   string
	labels string // pre-joined "k=v,k2=v2"
}

type Registry struct {
	mu       sync.Mutex
	counters map[counterKey]*int64
	gauges   map[counterKey]*int64
	buildInfo string
}

func New(version, commit string) *Registry {
	return &Registry{
		counters:  map[counterKey]*int64{},
		gauges:    map[counterKey]*int64{},
		buildInfo: fmt.Sprintf(`shortr_build_info{version=%q,commit=%q} 1`, version, commit),
	}
}

func labelStr(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(strconv.Quote(labels[k]))
	}
	return sb.String()
}

func (r *Registry) Inc(name string, n int64, labels map[string]string) {
	key := counterKey{name, labelStr(labels)}
	r.mu.Lock()
	p, ok := r.counters[key]
	if !ok {
		var v int64
		p = &v
		r.counters[key] = p
	}
	r.mu.Unlock()
	atomic.AddInt64(p, n)
}

func (r *Registry) Set(name string, v int64, labels map[string]string) {
	key := counterKey{name, labelStr(labels)}
	r.mu.Lock()
	p, ok := r.gauges[key]
	if !ok {
		var vv int64
		p = &vv
		r.gauges[key] = p
	}
	r.mu.Unlock()
	atomic.StoreInt64(p, v)
}

// --- convenience wrappers matching the click.Metrics interface ----------

func (r *Registry) IncClicksWritten(n int64)  { r.Inc("shortr_clicks_written_total", n, nil) }
func (r *Registry) IncClicksDropped(n int64)  { r.Inc("shortr_clicks_dropped_total", n, nil) }
func (r *Registry) IncClicksSpooled(n int64)  { r.Inc("shortr_clicks_spooled_total", n, nil) }
func (r *Registry) SetQueueDepth(n int)       { r.Set("shortr_clicks_queued", int64(n), nil) }
func (r *Registry) IncDBErrors(n int64)       { r.Inc("shortr_db_errors_total", n, nil) }

func (r *Registry) IncHTTPRequest(route, method string, status int) {
	r.Inc("shortr_http_requests_total", 1, map[string]string{"route": route, "method": method, "status": strconv.Itoa(status)})
}
func (r *Registry) IncRedirect(result string) {
	r.Inc("shortr_redirects_total", 1, map[string]string{"result": result})
}
func (r *Registry) IncCacheHit()  { r.Inc("shortr_cache_hits_total", 1, nil) }
func (r *Registry) IncCacheMiss() { r.Inc("shortr_cache_misses_total", 1, nil) }
func (r *Registry) SetCacheEntries(n int) { r.Set("shortr_cache_entries", int64(n), nil) }
func (r *Registry) IncRateLimited(scope string) {
	r.Inc("shortr_ratelimited_total", 1, map[string]string{"scope": scope})
}
func (r *Registry) SetLinksTotal(n int64) { r.Set("shortr_links_total", n, nil) }
func (r *Registry) SetUsersTotal(n int64) { r.Set("shortr_users_total", n, nil) }

// Render writes Prometheus text-exposition format.
func (r *Registry) Render() string {
	var sb strings.Builder
	sb.WriteString(r.buildInfo)
	sb.WriteByte('\n')

	r.mu.Lock()
	defer r.mu.Unlock()

	write := func(m map[counterKey]*int64) {
		keys := make([]counterKey, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].name != keys[j].name {
				return keys[i].name < keys[j].name
			}
			return keys[i].labels < keys[j].labels
		})
		for _, k := range keys {
			v := atomic.LoadInt64(m[k])
			if k.labels == "" {
				fmt.Fprintf(&sb, "%s %d\n", k.name, v)
			} else {
				fmt.Fprintf(&sb, "%s{%s} %d\n", k.name, k.labels, v)
			}
		}
	}
	write(r.counters)
	write(r.gauges)

	fmt.Fprintf(&sb, "go_goroutines %d\n", runtime.NumGoroutine())
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(&sb, "go_memstats_alloc_bytes %d\n", ms.Alloc)
	fmt.Fprintf(&sb, "go_memstats_sys_bytes %d\n", ms.Sys)
	fmt.Fprintf(&sb, "go_memstats_heap_objects %d\n", ms.HeapObjects)

	return sb.String()
}
