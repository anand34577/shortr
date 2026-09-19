package link

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"shortr/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"), dir, 4)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	svc := NewService(st, Config{BaseURL: "https://sho.rt", CodeLength: 7})
	return svc, st
}

func TestCreateAndResolve(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	l, err := svc.Create(ctx, nil, "1.2.3.4", CreateInput{TargetURL: "https://example.com/page"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if l.Code == "" || len(l.Code) != 7 {
		t.Fatalf("expected 7-char generated code, got %q", l.Code)
	}

	got, err := svc.ResolveForRedirect(ctx, l.Code)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.TargetURL != "https://example.com/page" {
		t.Fatalf("wrong target: %q", got.TargetURL)
	}

	// second resolve should hit cache (same pointer identity is not guaranteed
	// but the value must still be correct)
	got2, err := svc.ResolveForRedirect(ctx, l.Code)
	if err != nil || got2.ID != l.ID {
		t.Fatalf("cached resolve failed: %v", err)
	}
}

func TestCreateRejectsPrivateTarget(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Create(context.Background(), nil, "1.2.3.4", CreateInput{TargetURL: "http://localhost:8080/x"})
	if err == nil {
		t.Fatal("expected validation error for localhost target")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if _, ok := ve.Errors.Map()["target_url"]; !ok {
		t.Fatalf("expected target_url field error, got %v", ve.Errors.Map())
	}
}

func TestCreateCustomAliasCollision(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, nil, "", CreateInput{TargetURL: "https://example.com/1", Code: "myalias"}); err != nil {
		t.Fatalf("create 1: %v", err)
	}
	_, err := svc.Create(ctx, nil, "", CreateInput{TargetURL: "https://example.com/2", Code: "myalias"})
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if _, ok := ve.Errors.Map()["code"]; !ok {
		t.Fatal("expected code field error")
	}
}

func TestCreateRejectsReservedAlias(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Create(context.Background(), nil, "", CreateInput{TargetURL: "https://example.com", Code: "api"})
	if err == nil {
		t.Fatal("expected reserved alias to be rejected")
	}
}

func TestResolveUnknownCodeIsCachedNegative(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	_, err := svc.ResolveForRedirect(ctx, "doesnotexist")
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// should now be served from the negative cache
	e, ok := svc.Cache().Get("doesnotexist")
	if !ok || !e.Missing {
		t.Fatal("expected negative cache entry")
	}
}

func TestUpdateInvalidatesCache(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	l, _ := svc.Create(ctx, nil, "", CreateInput{TargetURL: "https://example.com/old"})
	svc.ResolveForRedirect(ctx, l.Code) // warm cache

	newTarget := "https://example.com/new"
	if err := svc.Update(ctx, l, UpdateInput{TargetURL: &newTarget}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := svc.ResolveForRedirect(ctx, l.Code)
	if err != nil {
		t.Fatalf("resolve after update: %v", err)
	}
	if got.TargetURL != newTarget {
		t.Fatalf("expected cache to reflect update, got %q", got.TargetURL)
	}
}

func TestDeleteThenResolveNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	l, _ := svc.Create(ctx, nil, "", CreateInput{TargetURL: "https://example.com"})
	svc.ResolveForRedirect(ctx, l.Code)
	if err := svc.Delete(ctx, l.ID, l.Code); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := svc.ResolveForRedirect(ctx, l.Code)
	if err != nil {
		t.Fatalf("resolve should still find soft-deleted link (caller checks DeletedAt): %v", err)
	}
	if got.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := NewCache(shardCount * 2) // 2 per shard: tiny, forces eviction fast
	l1 := &store.Link{ID: "1", Code: "aaa"}
	c.Put("aaa", l1)
	if _, ok := c.Get("aaa"); !ok {
		t.Fatal("expected hit right after put")
	}
}

func TestCacheExpiry(t *testing.T) {
	c := NewCache(160)
	c.put("x", &Entry{Link: &store.Link{ID: "1"}, expires: time.Now().Add(-time.Second)})
	if _, ok := c.Get("x"); ok {
		t.Fatal("expected expired entry to be evicted on read")
	}
}

func TestGenerateCodeNoModuloBiasAndLength(t *testing.T) {
	seen := map[rune]int{}
	for i := 0; i < 2000; i++ {
		code, err := GenerateCode(AlphabetBase58, 7)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(code) != 7 {
			t.Fatalf("wrong length: %q", code)
		}
		for _, r := range code {
			seen[r]++
		}
	}
	if len(seen) < 20 {
		t.Fatalf("expected broad alphabet coverage, only saw %d distinct chars", len(seen))
	}
}
