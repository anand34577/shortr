package click

import (
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// GeoDB wraps an optional MaxMind mmdb lookup. Nil-safe: if no database is
// configured, Lookup returns zero values and the feature is simply off (no
// outbound network call is ever made — PLAN.md §3.1).
type GeoDB struct {
	mu sync.RWMutex
	db *maxminddb.Reader
	path string
}

func OpenGeoDB(path string) (*GeoDB, error) {
	g := &GeoDB{path: path}
	if path == "" {
		return g, nil
	}
	db, err := maxminddb.Open(path)
	if err != nil {
		return g, err // caller logs a warning and continues with geo disabled
	}
	g.db = db
	return g, nil
}

func (g *GeoDB) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.db != nil {
		g.db.Close()
		g.db = nil
	}
}

// Reload re-opens the mmdb file, picking up an admin-provided update without
// a restart (PLAN.md §24.2 "GeoIP file updated on disk").
func (g *GeoDB) Reload() error {
	if g.path == "" {
		return nil
	}
	db, err := maxminddb.Open(g.path)
	if err != nil {
		return err
	}
	g.mu.Lock()
	old := g.db
	g.db = db
	g.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

type GeoResult struct {
	Country string
	Region  string
	City    string
}

type mmdbRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

func (g *GeoDB) Lookup(ip net.IP) GeoResult {
	g.mu.RLock()
	db := g.db
	g.mu.RUnlock()
	if db == nil || ip == nil {
		return GeoResult{}
	}
	var rec mmdbRecord
	if err := db.Lookup(ip, &rec); err != nil {
		return GeoResult{}
	}
	res := GeoResult{Country: rec.Country.ISOCode}
	if len(rec.Subdivisions) > 0 {
		res.Region = rec.Subdivisions[0].Names["en"]
	}
	res.City = rec.City.Names["en"]
	return res
}
