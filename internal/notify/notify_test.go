package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"shortr/internal/store"
)

func TestBuildMessageHasHeaders(t *testing.T) {
	msg := string(buildMessage("from@x.com", "to@x.com", "Hi", "body text"))
	if !contains(msg, "From: from@x.com") || !contains(msg, "To: to@x.com") || !contains(msg, "Subject: Hi") || !contains(msg, "body text") {
		t.Fatalf("message missing expected parts:\n%s", msg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestGotifySendSuccess(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("token")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "hello" {
			t.Errorf("unexpected title: %v", body["title"])
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	g := NewGotifySender(GotifyConfig{Enabled: true, URL: srv.URL, Token: "tok123"})
	if err := g.Send(context.Background(), "hello", "world", PriorityNormal); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotToken != "tok123" {
		t.Fatalf("expected token to be passed, got %q", gotToken)
	}
}

func TestGotifyDisabledIsNoop(t *testing.T) {
	g := NewGotifySender(GotifyConfig{Enabled: false})
	if err := g.Send(context.Background(), "x", "y", PriorityNormal); err != nil {
		t.Fatalf("disabled sender should no-op, got %v", err)
	}
}

func TestPrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"), dir, 4)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	p := LoadPrefs(ctx, st, "u1")
	if !p.enabled(p.Email, KindLinkExpiring) {
		t.Fatal("default should be enabled")
	}

	p.Email[string(KindLinkExpiring)] = false
	if err := SavePrefs(ctx, st, "u1", p); err != nil {
		t.Fatalf("save: %v", err)
	}
	p2 := LoadPrefs(ctx, st, "u1")
	if p2.enabled(p2.Email, KindLinkExpiring) {
		t.Fatal("expected disabled after save")
	}
	if !p2.enabled(p2.Email, KindBackupFailed) {
		t.Fatal("other kinds should remain enabled by default")
	}
}

func TestNotifyUserStoresInAppNotification(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"), dir, 4)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	ctx := context.Background()

	u := &store.User{Email: "a@example.com", Role: "user"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	n := New(st, NewSMTPSender(SMTPConfig{}), NewGotifySender(GotifyConfig{}), nil)
	n.NotifyUser(ctx, u.ID, u.Email, KindLinkExpiring, "Link expiring", "your link expires soon", nil)

	list, err := st.ListNotifications(ctx, u.ID, false, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 notification, got %d err=%v", len(list), err)
	}
	if list[0].Title != "Link expiring" {
		t.Fatalf("unexpected title: %q", list[0].Title)
	}
}
