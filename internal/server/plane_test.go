package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shortr/internal/config"
	"shortr/internal/store"
)

func splitMode(c *config.Config) {
	c.Listen, c.AdminListen = ":8080", ":8081"
}

// serve sends a request to one plane with optional cookies / bearer key.
func serve(h http.Handler, method, path string, cookies []*http.Cookie, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "203.0.113.9:1"
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// adminSetup creates the first admin through handler h and returns its
// session cookies, CSRF token and a links+stats API key.
func adminSetup(t *testing.T, env *testEnv, h http.Handler) ([]*http.Cookie, string, string) {
	t.Helper()
	orig := env.srv.handler
	env.srv.handler = h // env.do always targets srv.handler
	defer func() { env.srv.handler = orig }()

	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	var me meDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	rec = env.do(t, "POST", "/api/v1/apikeys", map[string]any{"name": "phone", "scopes": []string{"links:read", "links:write", "stats:read"}}, cookies, me.CSRFToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apikey: %d %s", rec.Code, rec.Body.String())
	}
	var k map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &k)
	return cookies, me.CSRFToken, k["key"].(string)
}

func TestSplitPlanes(t *testing.T) {
	env := newTestEnv(t, splitMode)
	pub, adm := env.srv.handler, env.srv.adminHandler
	if adm == nil {
		t.Fatal("admin handler not built in split mode")
	}
	cookies, _, key := adminSetup(t, env, adm)

	// public plane: UI, cookie sign-in, ops data and admin API are invisible
	for _, p := range []string{"/app/", "/app/login", "/auth/status", "/metrics", "/version", "/api/v1/users", "/api/v1/audit", "/api/v1/settings"} {
		if rec := serve(pub, "GET", p, nil, key); rec.Code != http.StatusNotFound {
			t.Errorf("public %s: want 404, got %d", p, rec.Code)
		}
	}
	if rec := serve(pub, "POST", "/auth/login", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("public login: want 404, got %d", rec.Code)
	}
	// a valid session cookie is ignored on the public plane
	if rec := serve(pub, "GET", "/api/v1/me", cookies, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("public cookie auth: want 401, got %d", rec.Code)
	}
	// ...but API keys work there (the mobile app path)
	if rec := serve(pub, "GET", "/api/v1/links", nil, key); rec.Code != http.StatusOK {
		t.Errorf("public api key: want 200, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := serve(pub, "GET", "/healthz", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("public healthz: %d", rec.Code)
	}
	if rec := serve(pub, "GET", "/", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("public root without ROOT_REDIRECT: want 404, got %d", rec.Code)
	}

	// admin plane: everything, including the UI entry point
	if rec := serve(adm, "GET", "/", nil, ""); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/app" {
		t.Errorf("admin root: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := serve(adm, "GET", "/api/v1/audit", cookies, ""); rec.Code != http.StatusOK {
		t.Errorf("admin audit via session: %d", rec.Code)
	}
	if rec := serve(adm, "GET", "/metrics", nil, ""); rec.Code == http.StatusNotFound {
		t.Error("admin metrics hidden")
	}
}

func TestSplitPublicAPIOff(t *testing.T) {
	env := newTestEnv(t, splitMode, func(c *config.Config) {
		c.PublicAPI = false
		c.RootRedirect = "https://home.example.com"
	})
	_, _, key := adminSetup(t, env, env.srv.adminHandler)
	if rec := serve(env.srv.handler, "GET", "/api/v1/links", nil, key); rec.Code != http.StatusNotFound {
		t.Errorf("public api disabled: want 404, got %d", rec.Code)
	}
	if rec := serve(env.srv.handler, "GET", "/", nil, ""); rec.Header().Get("Location") != "https://home.example.com" {
		t.Errorf("root redirect: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestSinglePortUnchanged(t *testing.T) {
	env := newTestEnv(t)
	if env.srv.adminHandler != nil {
		t.Fatal("single-port mode must not build an admin handler")
	}
	if rec := serve(env.srv.handler, "GET", "/", nil, ""); rec.Header().Get("Location") != "/app" {
		t.Errorf("root: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := serve(env.srv.handler, "GET", "/auth/status", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("auth status: %d", rec.Code)
	}
}

func TestAdminSettingsTakeEffect(t *testing.T) {
	env := newTestEnv(t) // env default: registration closed
	cookies, csrf, _ := adminSetup(t, env, env.srv.handler)
	reg := map[string]string{"email": "b@example.com", "password": "anotherlongpassword"}

	if rec := env.do(t, "POST", "/auth/register", reg, nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("register while closed: %d", rec.Code)
	}
	if rec := env.do(t, "PUT", "/api/v1/settings", map[string]any{"registration": "nope"}, cookies, csrf); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid registration value: want 400, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(t, "PUT", "/api/v1/settings", map[string]any{"registration": "open", "defaultRedirectStatus": 301}, cookies, csrf); rec.Code != http.StatusOK {
		t.Fatalf("save settings: %d %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(t, "POST", "/auth/register", reg, nil, ""); rec.Code != http.StatusCreated {
		t.Fatalf("register after opening: %d %s", rec.Code, rec.Body.String())
	}
	rec := env.do(t, "POST", "/api/v1/links", map[string]any{"targetUrl": "https://example.org"}, cookies, csrf)
	var l linkDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &l)
	if l.RedirectStatus != 301 {
		t.Errorf("default redirect status setting ignored: got %d (%s)", l.RedirectStatus, rec.Body.String())
	}
}

func TestAuditRecordsFailedLoginAndFilters(t *testing.T) {
	env := newTestEnv(t)
	cookies, _, _ := adminSetup(t, env, env.srv.handler)
	env.do(t, "POST", "/auth/login", map[string]string{"email": "a@example.com", "password": "wrong-password-here"}, nil, "")

	var out struct {
		Items []auditDTO `json:"items"`
	}
	rec := env.do(t, "GET", "/api/v1/audit?action=user.login_failed", nil, cookies, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) != 1 || out.Items[0].Action != "user.login_failed" {
		t.Fatalf("filtered audit: %s", rec.Body.String())
	}
	rec = env.do(t, "GET", "/api/v1/audit?action=apikey.", nil, cookies, "")
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) != 1 || out.Items[0].Action != "apikey.create" {
		t.Fatalf("prefix filter: %s", rec.Body.String())
	}
}

func TestSeriesZeroFilled(t *testing.T) {
	to := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	from := to.Add(-7 * 24 * time.Hour)
	got := toSeriesDTO([]store.TimeSeriesPoint{{Bucket: "2026-01-03", Clicks: 4}}, from, to, false)
	if len(got) != 7 || got[0].Bucket != "2026-01-01" || got[2].Clicks != 4 || got[6].Bucket != "2026-01-07" {
		t.Fatalf("daily fill: %+v", got)
	}
	got = toSeriesDTO(nil, to.Add(-24*time.Hour), to, true)
	if len(got) != 24 || got[0].Bucket != "2026-01-07T00" {
		t.Fatalf("hourly fill: len=%d first=%+v", len(got), got[0])
	}
}

func TestRealIPFallsBackToXFFWhenCustomHeaderMissing(t *testing.T) {
	_, proxy, _ := net.ParseCIDR("172.18.0.0/16")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "172.18.0.5:4000"
	r.Header.Set("X-Forwarded-For", "192.168.1.20")
	if ip := resolveClientIP(r, []*net.IPNet{proxy}, "CF-Connecting-IP"); ip.String() != "192.168.1.20" {
		t.Fatalf("VPN request via proxy: got %v", ip)
	}
	r.Header.Set("CF-Connecting-IP", "198.51.100.7")
	if ip := resolveClientIP(r, []*net.IPNet{proxy}, "CF-Connecting-IP"); ip.String() != "198.51.100.7" {
		t.Fatalf("tunnel request: got %v", ip)
	}
	r.RemoteAddr = "203.0.113.1:1" // untrusted peer: headers ignored entirely
	if ip := resolveClientIP(r, []*net.IPNet{proxy}, "CF-Connecting-IP"); ip.String() != "203.0.113.1" {
		t.Fatalf("spoofed headers honoured: got %v", ip)
	}
}

func TestClientConfigOnPublicPlane(t *testing.T) {
	env := newTestEnv(t, splitMode)
	rec := serve(env.srv.handler, "GET", "/api/v1/client-config", nil, "")
	var cc clientConfigDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &cc)
	if rec.Code != http.StatusOK || cc.Name != "shortr" || cc.BaseURL != "http://sho.rt" || cc.OIDC != nil {
		t.Fatalf("client-config: %d %s", rec.Code, rec.Body.String())
	}
}
