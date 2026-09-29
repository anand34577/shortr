package click

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"shortr/internal/store"
)

// The service is down when a click arrives, comes back later, and the backfill
// fills in the click that was recorded without a location.
func TestBackfillGeoAfterServiceRecovers(t *testing.T) {
	var up atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"countryCode":"AU","subdivision":"Queensland","city":"Brisbane"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "test.db"), dir, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	l := &store.Link{Code: "bf", TargetURL: "https://example.com"}
	if err := st.CreateLink(ctx, l); err != nil {
		t.Fatal(err)
	}

	w := NewWriter(st, &GeoDB{}, WriterConfig{
		IPMode:     "full",
		IPLocation: func(context.Context) (bool, string) { return true, srv.URL },
	}, nil, nil)

	// service down: the click is stored with its IP but no location
	c := w.enrich(Event{LinkID: l.ID, TS: time.Now(), RawIP: net.ParseIP("1.1.1.1")})
	if c.IP != "1.1.1.1" || c.Country != "" {
		t.Fatalf("enrich while down: ip=%q country=%q", c.IP, c.Country)
	}
	if err := st.InsertClicksBatch(ctx, []*store.Click{c}, true); err != nil {
		t.Fatal(err)
	}

	// still down: backfill is a no-op and must not error
	if err := w.BackfillGeo(ctx); err != nil {
		t.Fatal(err)
	}

	// service back, breaker window over: the backfill fills the click in
	up.Store(true)
	w.ipLoc.mu.Lock()
	w.ipLoc.downUntil = time.Time{}
	w.ipLoc.cache = nil
	w.ipLoc.mu.Unlock()
	w.backfillCursor = 0
	if err := w.BackfillGeo(ctx); err != nil {
		t.Fatal(err)
	}
	got, _, err := st.ListClicks(ctx, store.ClickFilter{LinkID: l.ID, Limit: 10})
	if err != nil || len(got) != 1 {
		t.Fatalf("list clicks: %v %d", err, len(got))
	}
	if got[0].Country != "AU" || got[0].City != "Brisbane" {
		t.Fatalf("click not backfilled: %+v", got[0])
	}
}

// While the service is down, an expired answer is still better than none.
func TestIPLookupServesStaleWhenDown(t *testing.T) {
	var up atomic.Bool
	up.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"countryCode":"DE","city":"Berlin"}`))
	}))
	defer srv.Close()
	l := &ipLookup{settings: func(context.Context) (bool, string) { return true, srv.URL }}
	ip := net.ParseIP("8.8.8.8")
	if g := l.Lookup(ip); g.Country != "DE" {
		t.Fatalf("first lookup: %+v", g)
	}
	up.Store(false)
	l.mu.Lock()
	e := l.cache["8.8.8.8"]
	e.exp = time.Now().Add(-time.Second) // expired
	l.cache["8.8.8.8"] = e
	l.mu.Unlock()
	if g := l.Lookup(ip); g.Country != "DE" {
		t.Fatalf("stale fallback: %+v", g)
	}
}
