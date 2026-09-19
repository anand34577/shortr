package store

import (
	"context"
	"testing"
	"time"
)

func TestUserCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u := &User{Email: "a@example.com", Name: "Alice", Role: "admin"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected id to be set")
	}

	got, err := s.GetUserByEmail(ctx, "a@example.com")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if got.Name != "Alice" || !got.IsAdmin() {
		t.Fatalf("unexpected user: %+v", got)
	}

	got.Name = "Alice B"
	if err := s.UpdateUser(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := s.GetUserByID(ctx, u.ID)
	if got2.Name != "Alice B" {
		t.Fatalf("update didn't persist: %+v", got2)
	}

	// duplicate email -> conflict
	dup := &User{Email: "a@example.com", Name: "Dup"}
	if err := s.CreateUser(ctx, dup); err == nil {
		t.Fatal("expected conflict error on duplicate email")
	}

	n, err := s.CountUsers(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count users: n=%d err=%v", n, err)
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetUserByID(ctx, u.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLinkCRUDAndCaseInsensitiveLookup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	l := &Link{Code: "AbC123", TargetURL: "https://example.com/page", RedirectStatus: 302, PassQuery: true}
	if err := s.CreateLink(ctx, l); err != nil {
		t.Fatalf("create: %v", err)
	}

	exact, err := s.GetLinkByCode(ctx, "AbC123")
	if err != nil || exact.ID != l.ID {
		t.Fatalf("exact lookup failed: %v", err)
	}

	ci, err := s.GetLinkByCode(ctx, "abc123")
	if err != nil || ci.ID != l.ID {
		t.Fatalf("case-insensitive lookup failed: %v", err)
	}

	exists, err := s.CodeExists(ctx, "ABC123")
	if err != nil || !exists {
		t.Fatalf("CodeExists should be true: exists=%v err=%v", exists, err)
	}

	// second link with same code but different case must collide (case-insensitive unique index)
	dup := &Link{Code: "ABC123", TargetURL: "https://example.com/other"}
	if err := s.CreateLink(ctx, dup); err == nil {
		t.Fatal("expected conflict on case-insensitive duplicate code")
	}

	if err := s.SoftDeleteLink(ctx, l.ID, time.Now()); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	got, _ := s.GetLinkByID(ctx, l.ID)
	if got.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set")
	}

	if err := s.RestoreLink(ctx, l.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, _ = s.GetLinkByID(ctx, l.ID)
	if got.DeletedAt != nil {
		t.Fatal("expected deleted_at to be cleared")
	}
}

func TestClickInsertAndCount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	l := &Link{Code: "clk1", TargetURL: "https://example.com"}
	if err := s.CreateLink(ctx, l); err != nil {
		t.Fatalf("create link: %v", err)
	}

	clicks := []*Click{
		{LinkID: l.ID, TS: time.Now(), IP: "1.2.3.0", Device: "desktop", Country: "US"},
		{LinkID: l.ID, TS: time.Now(), IP: "1.2.3.1", Device: "mobile", Country: "IN"},
		{LinkID: l.ID, TS: time.Now(), IP: "1.2.3.2", Device: "bot", IsBot: true, Country: "US"},
	}
	if err := s.InsertClicksBatch(ctx, clicks, false); err != nil {
		t.Fatalf("insert clicks: %v", err)
	}

	got, _ := s.GetLinkByID(ctx, l.ID)
	if got.ClickCount != 2 {
		t.Fatalf("expected click_count=2 (bots excluded), got %d", got.ClickCount)
	}

	total, err := s.TotalClicks(ctx, l.ID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true)
	if err != nil || total != 3 {
		t.Fatalf("total with bots: n=%d err=%v", total, err)
	}

	rows, err := s.ClicksBreakdown(ctx, l.ID, "country", time.Now().Add(-time.Hour), time.Now().Add(time.Hour), 10)
	if err != nil {
		t.Fatalf("breakdown: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected breakdown rows")
	}
}

func TestSettingsUpsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SetSetting(ctx, "site_name", "Shortr", "sys"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, ok, err := s.GetSetting(ctx, "site_name")
	if err != nil || !ok || v != "Shortr" {
		t.Fatalf("get: v=%q ok=%v err=%v", v, ok, err)
	}
	if err := s.SetSetting(ctx, "site_name", "Shortr2", "sys"); err != nil {
		t.Fatalf("re-set: %v", err)
	}
	v, _, _ = s.GetSetting(ctx, "site_name")
	if v != "Shortr2" {
		t.Fatalf("expected upsert to overwrite, got %q", v)
	}
}
