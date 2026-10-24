package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Walking every page under each sort must visit every link exactly once.
func TestListLinksPaginatesEverySort(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 7; i++ {
		// shared click counts and titles force the id tie-break
		l := &Link{Code: fmt.Sprintf("pg%d", i), TargetURL: "https://example.com", Title: fmt.Sprintf("t%d", i%2), ClickCount: int64(i % 3)}
		if err := s.CreateLink(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	for _, sort := range []string{"created_at", "clicks", "title"} {
		for _, order := range []string{"asc", "desc"} {
			seen := map[string]bool{}
			cursor := ""
			for page := 0; ; page++ {
				links, next, err := s.ListLinks(ctx, LinkFilter{Sort: sort, Order: order, Limit: 2, Cursor: cursor})
				if err != nil {
					t.Fatalf("%s %s: %v", sort, order, err)
				}
				for _, l := range links {
					if seen[l.ID] {
						t.Fatalf("%s %s: %s repeated", sort, order, l.Code)
					}
					seen[l.ID] = true
				}
				if next == "" || page > 10 {
					break
				}
				cursor = next
			}
			if len(seen) != 7 {
				t.Fatalf("%s %s: saw %d of 7 links", sort, order, len(seen))
			}
		}
	}
	if _, _, err := s.ListLinks(ctx, LinkFilter{Cursor: "!!"}); err != ErrInvalidCursor {
		t.Fatalf("bad cursor: got %v", err)
	}
}

func TestListLinksSearchIsLiteralAndCaseInsensitive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, l := range []*Link{
		{Code: "a1", TargetURL: "https://example.com", Title: "Save 50% Today", Tags: []string{"Promo"}},
		{Code: "a2", TargetURL: "https://example.com", Title: "Save 500 today"},
		{Code: "my_link", TargetURL: "https://example.com"},
		{Code: "myxlink", TargetURL: "https://example.com"},
	} {
		if err := s.CreateLink(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	count := func(f LinkFilter) int {
		links, _, err := s.ListLinks(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return len(links)
	}
	if n := count(LinkFilter{Query: "50%"}); n != 1 {
		t.Fatalf(`"50%%" matched %d links, want 1`, n)
	}
	if n := count(LinkFilter{Query: "my_"}); n != 1 {
		t.Fatalf(`"my_" matched %d links, want 1`, n)
	}
	if n := count(LinkFilter{Query: "SAVE"}); n != 2 {
		t.Fatalf(`"SAVE" matched %d links, want 2`, n)
	}
	if n := count(LinkFilter{Tag: "Promo"}); n != 1 {
		t.Fatalf("tag matched %d links, want 1", n)
	}
}

// Clicks sharing a millisecond must not be dropped at a page boundary.
func TestListClicksPagesThroughSameTimestamp(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	l := &Link{Code: "burst", TargetURL: "https://example.com"}
	if err := s.CreateLink(ctx, l); err != nil {
		t.Fatal(err)
	}
	ts := time.Now()
	var clicks []*Click
	for i := 0; i < 5; i++ {
		clicks = append(clicks, &Click{LinkID: l.ID, TS: ts})
	}
	if err := s.InsertClicksBatch(ctx, clicks, false); err != nil {
		t.Fatal(err)
	}
	total, cursor := 0, ""
	for page := 0; page < 10; page++ {
		got, next, err := s.ListClicks(ctx, ClickFilter{LinkID: l.ID, Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		total += len(got)
		if next == "" {
			break
		}
		cursor = next
	}
	if total != 5 {
		t.Fatalf("paged through %d of 5 clicks", total)
	}
}
