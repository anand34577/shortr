package iploc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.TrimPrefix(r.URL.Path, "/")
		if ip != "1.1.1.1" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(Result{IP: ip, ASN: 13335, ASNOrganization: "Cloudflare, Inc."})
	}))
	defer srv.Close()

	res, err := Lookup(context.Background(), srv.URL, "1.1.1.1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if res.ASN != 13335 || res.ASNOrganization != "Cloudflare, Inc." {
		t.Fatalf("unexpected result: %+v", res)
	}

	if _, err := Lookup(context.Background(), srv.URL, "not-an-ip"); err == nil {
		t.Fatal("expected error for invalid IP")
	}
	if _, err := Lookup(context.Background(), "", "1.1.1.1"); err == nil {
		t.Fatal("expected error for empty base URL")
	}
	if _, err := Lookup(context.Background(), srv.URL, "8.8.8.8"); err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}
