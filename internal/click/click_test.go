package click

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"shortr/internal/store"
)

func TestParseUABot(t *testing.T) {
	cases := []struct {
		ua      string
		wantBot bool
		wantDev string
	}{
		{"", true, "other"},
		{"WhatsApp/2.23.20.0", true, "bot"},
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true, "bot"},
		{"curl/8.1.0", true, "bot"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", false, "desktop"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15", false, "mobile"},
	}
	for _, c := range cases {
		p := ParseUA(c.ua)
		if p.IsBot != c.wantBot {
			t.Errorf("ua=%q: IsBot=%v want %v", c.ua, p.IsBot, c.wantBot)
		}
		if p.Device != c.wantDev {
			t.Errorf("ua=%q: Device=%q want %q", c.ua, p.Device, c.wantDev)
		}
	}
}

func TestApplyIPModeAnonymize(t *testing.T) {
	ip := net.ParseIP("203.0.113.42")
	got := ApplyIPMode("anonymize", ip, nil)
	if got != "203.0.113.0" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyIPModeHashDeterministicPerDay(t *testing.T) {
	ip := net.ParseIP("203.0.113.42")
	secret := []byte("s3cr3t")
	a := ApplyIPMode("hash", ip, secret)
	b := ApplyIPMode("hash", ip, secret)
	if a != b {
		t.Fatal("hash mode should be deterministic within the same day")
	}
	if len(a) != 32 {
		t.Fatalf("expected 32-char hash, got %d", len(a))
	}
}

func TestApplyIPModeFullAndNone(t *testing.T) {
	ip := net.ParseIP("203.0.113.42")
	if got := ApplyIPMode("full", ip, nil); got != "203.0.113.42" {
		t.Fatalf("full: got %q", got)
	}
	if got := ApplyIPMode("none", ip, nil); got != "" {
		t.Fatalf("none: got %q", got)
	}
}

func TestFirstLangTag(t *testing.T) {
	if got := FirstLangTag("en-US,en;q=0.9,fr;q=0.8"); got != "en-US" {
		t.Fatalf("got %q", got)
	}
	if got := FirstLangTag(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestWriterEnqueueFlushAndSpoolReplay(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open("sqlite", dbPath, dir, 4)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	l := &store.Link{Code: "wtest", TargetURL: "https://example.com"}
	if err := st.CreateLink(context.Background(), l); err != nil {
		t.Fatalf("create link: %v", err)
	}

	geo, _ := OpenGeoDB("")
	w := NewWriter(st, geo, WriterConfig{IPMode: "anonymize", SpoolDir: filepath.Join(dir, "spool"), FlushInterval: 20 * time.Millisecond}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	for i := 0; i < 5; i++ {
		ok := w.Enqueue(Event{LinkID: l.ID, TS: time.Now(), RawIP: net.ParseIP("1.2.3.4"), UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"})
		if !ok {
			t.Fatal("enqueue should not drop under low load")
		}
	}
	time.Sleep(100 * time.Millisecond) // let the flush ticker fire
	w.Close()
	cancel()
	<-done

	got, _ := st.GetLinkByID(context.Background(), l.ID)
	if got.ClickCount != 5 {
		t.Fatalf("expected click_count=5, got %d", got.ClickCount)
	}
}
