package click

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"shortr/internal/store"
)

type WriterConfig struct {
	IPMode        string
	Secret        []byte
	CountBots     bool
	SpoolDir      string
	BatchSize     int
	FlushInterval time.Duration
	QueueSize     int
}

func (c *WriterConfig) setDefaults() {
	if c.BatchSize <= 0 {
		c.BatchSize = 500
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 250 * time.Millisecond
	}
	if c.QueueSize <= 0 {
		c.QueueSize = 10000
	}
}

// Writer consumes click events off a buffered channel, enriches them
// (UA/Geo/IP-mode), and batches inserts into the store. On sustained DB
// failure it spools batches to disk as JSONL and keeps accepting new events
// — a redirect never blocks on this.
type Writer struct {
	cfg     WriterConfig
	store   *store.Store
	geo     *GeoDB
	metrics Metrics
	log     *slog.Logger

	ch   chan Event
	done chan struct{}
}

func NewWriter(st *store.Store, geo *GeoDB, cfg WriterConfig, m Metrics, log *slog.Logger) *Writer {
	cfg.setDefaults()
	if m == nil {
		m = noopMetrics{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Writer{
		cfg: cfg, store: st, geo: geo, metrics: m, log: log,
		ch:   make(chan Event, cfg.QueueSize),
		done: make(chan struct{}),
	}
}

// Enqueue never blocks. Returns false (and the caller/metrics should count a
// drop) if the queue is full.
func (w *Writer) Enqueue(ev Event) bool {
	select {
	case w.ch <- ev:
		w.metrics.SetQueueDepth(len(w.ch))
		return true
	default:
		w.metrics.IncClicksDropped(1)
		return false
	}
}

// Run drains events until ctx is cancelled, then flushes whatever remains
// (bounded by the caller's shutdown timeout) and returns. Callers that need
// to know when the final flush has actually landed (e.g. graceful shutdown)
// should wait on Done() rather than sleeping a fixed duration.
func (w *Writer) Run(ctx context.Context) {
	defer close(w.done)
	if w.cfg.SpoolDir != "" {
		w.replaySpool(ctx)
	}
	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]*store.Click, 0, w.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flush(context.Background(), batch)
		batch = make([]*store.Click, 0, w.cfg.BatchSize)
	}

	for {
		select {
		case ev, ok := <-w.ch:
			if !ok {
				flush()
				return
			}
			if c := w.safeEnrich(ev); c != nil {
				batch = append(batch, c)
			}
			w.metrics.SetQueueDepth(len(w.ch))
			if len(batch) >= w.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// drain remaining buffered events (best-effort, non-blocking) then flush
			for {
				select {
				case ev, ok := <-w.ch:
					if !ok { // closed and drained: stop, don't spin on zero values
						flush()
						return
					}
					if c := w.safeEnrich(ev); c != nil {
						batch = append(batch, c)
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

// Close signals no more events will be enqueued; Run will flush and return.
func (w *Writer) Close() { close(w.ch) }

// Done is closed once Run has fully returned (final flush landed or spooled).
// Callers doing a graceful shutdown should wait on this instead of sleeping
// a fixed duration, which can cut a slow flush short and lose events.
func (w *Writer) Done() <-chan struct{} { return w.done }

// safeEnrich isolates a panic in enrich (malformed GeoIP record, UA-parse
// edge case, ...) to a single event instead of killing the writer goroutine
// forever, which would otherwise silently drop every click from then on.
func (w *Writer) safeEnrich(ev Event) (c *store.Click) {
	defer func() {
		if rec := recover(); rec != nil {
			w.metrics.IncClicksDropped(1)
			w.log.Error("panic enriching click event; dropping event", "panic", rec, "link_id", ev.LinkID)
			c = nil
		}
	}()
	return w.enrich(ev)
}

func (w *Writer) enrich(ev Event) *store.Click {
	geo := w.geo.Lookup(ev.RawIP) // must run on the raw IP, before anonymisation
	ua := ParseUA(ev.UserAgent)
	c := &store.Click{
		LinkID:         ev.LinkID,
		TS:             ev.TS,
		IP:             ApplyIPMode(w.cfg.IPMode, ev.RawIP, w.cfg.Secret),
		IPVersion:      IPVersion(ev.RawIP),
		Country:        geo.Country,
		Region:         geo.Region,
		City:           geo.City,
		Referrer:       truncate(ev.Referrer, 512),
		ReferrerHost:   refHost(ev.Referrer),
		UserAgent:      truncate(ev.UserAgent, 512),
		Device:         ua.Device,
		OS:             ua.OS,
		OSVersion:      ua.OSVersion,
		Browser:        ua.Browser,
		BrowserVersion: ua.BrowserVersion,
		IsBot:          ua.IsBot,
		Lang:           FirstLangTag(ev.AcceptLanguage),
		UTMSource:      ev.UTMSource,
		UTMMedium:      ev.UTMMedium,
		UTMCampaign:    ev.UTMCampaign,
		UTMTerm:        ev.UTMTerm,
		UTMContent:     ev.UTMContent,
		QS:             truncate(ev.QueryString, 256),
	}
	return c
}

func (w *Writer) flush(ctx context.Context, batch []*store.Click) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var err error
	for attempt, backoff := range []time.Duration{0, 100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second} {
		if attempt > 0 {
			time.Sleep(backoff)
		}
		err = w.store.InsertClicksBatch(ctx, batch, w.cfg.CountBots)
		if err == nil {
			w.metrics.IncClicksWritten(int64(len(batch)))
			return
		}
		w.metrics.IncDBErrors(1)
	}
	w.log.Warn("click batch write failed after retries; spooling to disk", "count", len(batch), "error", err)
	w.spool(batch)
}

// --- disk spool (survives DB outages) ---------------------------------

func (w *Writer) spool(batch []*store.Click) {
	if w.cfg.SpoolDir == "" {
		w.metrics.IncClicksDropped(int64(len(batch)))
		w.log.Error("click spool directory not configured; dropping batch", "count", len(batch))
		return
	}
	if err := os.MkdirAll(w.cfg.SpoolDir, 0o700); err != nil {
		w.metrics.IncClicksDropped(int64(len(batch)))
		w.log.Error("cannot create spool directory; dropping batch", "error", err)
		return
	}
	name := filepath.Join(w.cfg.SpoolDir, time.Now().UTC().Format("20060102T150405.000000000")+".jsonl")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		w.metrics.IncClicksDropped(int64(len(batch)))
		w.log.Error("cannot open spool file; dropping batch", "error", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, c := range batch {
		if err := enc.Encode(c); err != nil {
			w.log.Error("spool encode error", "error", err)
		}
	}
	w.metrics.IncClicksSpooled(int64(len(batch)))
}

// replaySpool is called once at startup: if the DB was down at last exit,
// spooled files get replayed now that we're connected again.
func (w *Writer) replaySpool(ctx context.Context) {
	entries, err := os.ReadDir(w.cfg.SpoolDir)
	if err != nil {
		return // no spool dir yet, nothing to do
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(w.cfg.SpoolDir, name)
		if w.replayFile(ctx, path) {
			os.Remove(path)
		}
	}
}

func (w *Writer) replayFile(ctx context.Context, path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var batch []*store.Click
	for {
		var c store.Click
		if err := dec.Decode(&c); err != nil {
			break
		}
		batch = append(batch, &c)
	}
	if len(batch) == 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := w.store.InsertClicksBatch(ctx, batch, w.cfg.CountBots); err != nil {
		w.log.Warn("spool replay failed, will retry next start", "file", path, "error", err)
		return false
	}
	w.log.Info("replayed spooled clicks", "file", path, "count", len(batch))
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func refHost(referrer string) string {
	if referrer == "" {
		return ""
	}
	u, err := url.Parse(referrer)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
