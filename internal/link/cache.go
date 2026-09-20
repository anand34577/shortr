package link

import (
	"container/list"
	"hash/fnv"
	"sync"
	"time"

	"shortr/internal/store"
)

const shardCount = 16
const defaultTTL = 5 * time.Minute
const negativeTTL = 60 * time.Second

// Entry is the fully-resolved data the redirect handler needs — no DB touch
// on a cache hit.
type Entry struct {
	Link    *store.Link
	Missing bool // negative-cache marker for unknown codes
	expires time.Time
}

type shard struct {
	mu    sync.Mutex
	items map[string]*list.Element // code -> node
	order *list.List               // front = most recently used
	cap   int
}

type node struct {
	code  string
	entry *Entry
}

// Cache is a sharded LRU with per-entry TTL and a negative cache for unknown
// codes (defends against enumeration/scanning storms hitting the DB).
type Cache struct {
	shards   [shardCount]*shard
	capacity int // per-shard capacity
}

func NewCache(totalCapacity int) *Cache {
	if totalCapacity <= 0 {
		totalCapacity = 50000
	}
	perShard := totalCapacity / shardCount
	if perShard < 16 {
		perShard = 16
	}
	c := &Cache{capacity: perShard}
	for i := range c.shards {
		c.shards[i] = &shard{items: make(map[string]*list.Element), order: list.New(), cap: perShard}
	}
	return c
}

func (c *Cache) shardFor(code string) *shard {
	h := fnv.New32a()
	h.Write([]byte(code))
	return c.shards[h.Sum32()%shardCount]
}

// Codes are case-insensitive in the database (unique index on LOWER(code)),
// so every cache operation keys on the lowercased code: one entry per link
// regardless of the casing a visitor used, and one Invalidate clears them all.
func (c *Cache) Get(code string) (*Entry, bool) {
	code = toLower(code)
	sh := c.shardFor(code)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	el, ok := sh.items[code]
	if !ok {
		return nil, false
	}
	n := el.Value.(*node)
	if time.Now().After(n.entry.expires) {
		sh.order.Remove(el)
		delete(sh.items, code)
		return nil, false
	}
	sh.order.MoveToFront(el)
	return n.entry, true
}

func (c *Cache) Put(code string, l *store.Link) {
	c.put(code, &Entry{Link: l, expires: time.Now().Add(defaultTTL)})
}

func (c *Cache) PutMissing(code string) {
	c.put(code, &Entry{Missing: true, expires: time.Now().Add(negativeTTL)})
}

func (c *Cache) put(code string, e *Entry) {
	code = toLower(code)
	sh := c.shardFor(code)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if el, ok := sh.items[code]; ok {
		el.Value.(*node).entry = e
		sh.order.MoveToFront(el)
		return
	}
	el := sh.order.PushFront(&node{code: code, entry: e})
	sh.items[code] = el
	if sh.order.Len() > sh.cap {
		back := sh.order.Back()
		if back != nil {
			sh.order.Remove(back)
			delete(sh.items, back.Value.(*node).code)
		}
	}
}

// Invalidate removes a code from the cache (positive or negative entry) —
// called on every link mutation for write-through consistency.
func (c *Cache) Invalidate(code string) {
	code = toLower(code)
	sh := c.shardFor(code)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if el, ok := sh.items[code]; ok {
		sh.order.Remove(el)
		delete(sh.items, code)
	}
}

func (c *Cache) Len() int {
	n := 0
	for _, sh := range c.shards {
		sh.mu.Lock()
		n += sh.order.Len()
		sh.mu.Unlock()
	}
	return n
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
