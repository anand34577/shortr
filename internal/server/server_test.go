package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"shortr/internal/click"
	"shortr/internal/config"
	"shortr/internal/link"
	"shortr/internal/metrics"
	"shortr/internal/store"
)

type testEnv struct {
	srv *Server
	st  *store.Store
	cw  *click.Writer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "t.db"), dir, 4)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := &config.Config{
		BaseURL: "http://sho.rt", BaseHost: "sho.rt", CookieSecure: false, SessionTTL: time.Hour,
		CodeLength: 7, CodeAlphabet: "base58", MaxURLLength: 2048, DefaultRedirectCode: 302,
		RateLimitRedirect: "1000/1s", RateLimitAPI: "1000/1s", RateLimitAuth: "1000/1s",
		SecretKey: []byte("0123456789abcdef0123456789abcdef"),
	}

	linkSvc := link.NewService(st, link.Config{BaseURL: cfg.BaseURL, BaseHost: cfg.BaseHost, CodeLength: 7, MaxURLLength: 2048, DefaultRedirectStatus: 302})
	geo, _ := click.OpenGeoDB("")
	m := metrics.New("test", "test")
	cw := click.NewWriter(st, geo, click.WriterConfig{IPMode: "anonymize", SpoolDir: filepath.Join(dir, "spool"), FlushInterval: 10 * time.Millisecond}, m, nil)
	go cw.Run(t.Context())
	t.Cleanup(cw.Close)

	srv := New(Deps{Config: cfg, Store: st, Links: linkSvc, ClickWriter: cw, Metrics: m, Version: "test"})
	return &testEnv{srv: srv, st: st, cw: cw}
}

func (e *testEnv) do(t *testing.T, method, path string, body any, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.RemoteAddr = "203.0.113.5:12345"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	e.srv.handler.ServeHTTP(rec, req)
	return rec
}

func TestSetupLoginCreateRedirectStats(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { env.srv.Shutdown(t.Context()) })

	// 1. setup
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "admin@example.com", "password": "correcthorsebatterystaple", "name": "Admin"}, nil, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup: status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()

	// 2. get /api/v1/me for csrf token
	rec = env.do(t, "GET", "/api/v1/me", nil, cookies, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("me: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var me meDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.Role != "admin" {
		t.Fatalf("expected admin role, got %q", me.Role)
	}
	csrf := me.CSRFToken
	if csrf == "" {
		t.Fatal("expected csrf token")
	}

	// 3. create a link without csrf token -> should be rejected
	rec = env.do(t, "POST", "/api/v1/links", map[string]string{"targetUrl": "https://example.com/hello"}, cookies, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected CSRF rejection, got status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 4. create a link with csrf token -> success
	rec = env.do(t, "POST", "/api/v1/links", map[string]string{"targetUrl": "https://example.com/hello", "code": "hello1"}, cookies, csrf)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create link: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var l linkDTO
	json.Unmarshal(rec.Body.Bytes(), &l) //nolint:errcheck
	if l.Code != "hello1" {
		t.Fatalf("expected code hello1, got %q", l.Code)
	}

	// 5. hit the redirect (with a real browser UA — an empty one is treated
	// as a bot and correctly excluded from click_count)
	req := httptest.NewRequest("GET", "/hello1", nil)
	req.RemoteAddr = "203.0.113.9:1111"
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
	rec2 := httptest.NewRecorder()
	env.srv.handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusFound {
		t.Fatalf("redirect: status=%d", rec2.Code)
	}
	if loc := rec2.Header().Get("Location"); loc != "https://example.com/hello" {
		t.Fatalf("unexpected location: %q", loc)
	}

	// 6. wait for the click to be flushed, then check click_count via API
	time.Sleep(150 * time.Millisecond)
	rec = env.do(t, "GET", "/api/v1/links/"+l.ID, nil, cookies, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get link: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got linkDTO
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.ClickCount != 1 {
		t.Fatalf("expected click_count=1, got %d", got.ClickCount)
	}

	// 7. stats endpoint returns the click
	rec = env.do(t, "GET", "/api/v1/links/"+l.ID+"/stats", nil, cookies, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var stats linkStatsResp
	json.Unmarshal(rec.Body.Bytes(), &stats) //nolint:errcheck
	if stats.Totals.Clicks != 1 {
		t.Fatalf("expected totals.clicks=1, got %d", stats.Totals.Clicks)
	}
}

func TestSetupOnlyOnce(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("first setup: %d %s", rec.Code, rec.Body.String())
	}
	rec = env.do(t, "POST", "/auth/setup", map[string]string{"email": "b@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on second setup, got %d", rec.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	env := newTestEnv(t)
	env.do(t, "POST", "/auth/setup", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	rec := env.do(t, "POST", "/auth/login", map[string]string{"email": "a@example.com", "password": "wrongpassword"}, nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRedirectUnknownCodeIs404(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("GET", "/doesnotexist", nil)
	req.RemoteAddr = "203.0.113.9:1111"
	rec := httptest.NewRecorder()
	env.srv.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPrivateTargetRejected(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	cookies := rec.Result().Cookies()
	rec = env.do(t, "GET", "/api/v1/me", nil, cookies, "")
	var me meDTO
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck

	rec = env.do(t, "POST", "/api/v1/links", map[string]string{"targetUrl": "http://169.254.169.254/latest/meta-data/"}, cookies, me.CSRFToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected validation error for link-local target, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyAuthWorks(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	cookies := rec.Result().Cookies()
	rec = env.do(t, "GET", "/api/v1/me", nil, cookies, "")
	var me meDTO
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck

	rec = env.do(t, "POST", "/api/v1/apikeys", map[string]string{"name": "test key"}, cookies, me.CSRFToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created) //nolint:errcheck
	rawKey, _ := created["key"].(string)
	if rawKey == "" {
		t.Fatal("expected raw key in response")
	}

	req := httptest.NewRequest("GET", "/api/v1/links", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.RemoteAddr = "203.0.113.5:1"
	rec2 := httptest.NewRecorder()
	env.srv.handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("api key auth: status=%d body=%s", rec2.Code, rec2.Body.String())
	}
}
