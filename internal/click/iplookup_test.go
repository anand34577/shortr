package click

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestIPLookupCachesAndSkipsPrivate(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/1.1.1.1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"ip":"1.1.1.1","countryCode":"AU","subdivision":"Queensland","city":"Brisbane"}`))
	}))
	defer srv.Close()
	l := &ipLookup{settings: func(context.Context) (bool, string) { return true, srv.URL }}

	for i := 0; i < 2; i++ {
		if g := l.Lookup(net.ParseIP("1.1.1.1")); g.Country != "AU" || g.City != "Brisbane" || g.Region != "Queensland" {
			t.Fatalf("got %+v", g)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1 (cached)", hits.Load())
	}
	if g := l.Lookup(net.ParseIP("192.168.1.5")); g != (GeoResult{}) || hits.Load() != 1 {
		t.Fatal("private IP must not be looked up")
	}
}
