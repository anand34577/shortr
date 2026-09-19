package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open("sqlite", filepath.Join(dir, "test.db"), dir, 4)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAndMigrate(t *testing.T) {
	s := newTestStore(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	var n int
	if err := s.write.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 migration applied, got %d", n)
	}
	// re-open should be idempotent (no duplicate-apply errors) after closing the lock
}

func TestLockPreventsSecondInstance(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open("sqlite", filepath.Join(dir, "test.db"), dir, 4)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s1.Close()
	_, err = Open("sqlite", filepath.Join(dir, "test.db"), dir, 4)
	if err == nil {
		t.Fatal("expected second Open to fail due to lock file")
	}
}
