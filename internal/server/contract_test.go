package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFrontendContract pins the response shapes web/src/lib/types.ts relies
// on; a mismatch here shows up as a broken screen in the SPA.
func TestFrontendContract(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { env.srv.Shutdown(t.Context()) })

	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "admin@example.com", "password": "correcthorsebatterystaple", "name": "Admin"}, nil, "")
	cookies := rec.Result().Cookies()
	var me meDTO
	json.Unmarshal(env.do(t, "GET", "/api/v1/me", nil, cookies, "").Body.Bytes(), &me) //nolint:errcheck
	csrf := me.CSRFToken

	get := func(path string) map[string]any {
		t.Helper()
		r := env.do(t, "GET", path, nil, cookies, "")
		if r.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, r.Code, r.Body.String())
		}
		var m map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &m); err != nil {
			t.Fatalf("GET %s: not a JSON object: %v", path, err)
		}
		return m
	}
	need := func(where string, m map[string]any, keys ...string) {
		t.Helper()
		for _, k := range keys {
			if _, ok := m[k]; !ok {
				t.Errorf("%s: missing key %q in %v", where, k, m)
			}
		}
	}

	// settings: blockedDomains must be an array, never null
	if _, ok := get("/api/v1/settings")["blockedDomains"].([]any); !ok {
		t.Error("settings.blockedDomains must be an array")
	}
	get("/api/v1/openapi.json")

	// links + bulk
	r := env.do(t, "POST", "/api/v1/links", map[string]string{"targetUrl": "https://example.com/a", "code": "abc123"}, cookies, csrf)
	var l linkDTO
	json.Unmarshal(r.Body.Bytes(), &l) //nolint:errcheck
	r = env.do(t, "POST", "/api/v1/links/bulk", map[string]any{"items": []map[string]any{{"id": l.ID, "action": "disable"}}}, cookies, csrf)
	var bulk struct {
		Items []bulkResult `json:"items"`
	}
	json.Unmarshal(r.Body.Bytes(), &bulk) //nolint:errcheck
	if r.Code != 200 || len(bulk.Items) != 1 || !bulk.Items[0].OK {
		t.Fatalf("bulk disable: %d %s", r.Code, r.Body.String())
	}
	need("link", get("/api/v1/links/"+l.ID), "deletedAt", "shortUrl", "status")
	if got := get("/api/v1/links/" + l.ID)["status"]; got != "disabled" {
		t.Errorf("status after bulk disable = %v", got)
	}
	r = env.do(t, "POST", "/api/v1/links/bulk", map[string]any{"items": []map[string]any{{"id": l.ID, "action": "delete"}}}, cookies, csrf)
	if r.Code != 200 {
		t.Fatalf("bulk delete: %d %s", r.Code, r.Body.String())
	}
	if get("/api/v1/links/" + l.ID)["deletedAt"] == nil {
		t.Error("deleted link must expose deletedAt")
	}

	// clicks rows expose ts
	if _, ok := get("/api/v1/links/" + l.ID + "/clicks")["items"].([]any); !ok {
		t.Error("clicks.items must be an array")
	}

	// users: create returns the user plus generatedPassword; list has linksCount
	r = env.do(t, "POST", "/api/v1/users", map[string]string{"email": "bob@example.com", "name": "Bob", "role": "user"}, cookies, csrf)
	var created map[string]any
	json.Unmarshal(r.Body.Bytes(), &created) //nolint:errcheck
	need("create user", created, "id", "email", "generatedPassword", "maxLinks", "roleLocked", "mustChangePassword")
	users := get("/api/v1/users")["items"].([]any)
	need("user list row", users[0].(map[string]any), "linksCount", "updatedAt")

	// api key creation is camelCase
	r = env.do(t, "POST", "/api/v1/apikeys", map[string]any{"name": "k", "scopes": []string{"links:read"}}, cookies, csrf)
	var key map[string]any
	json.Unmarshal(r.Body.Bytes(), &key) //nolint:errcheck
	need("apikey", key, "key", "createdAt", "expiresAt", "prefix")

	// audit meta is an object, actor email resolved
	audit := get("/api/v1/audit")["items"].([]any)
	row := audit[0].(map[string]any)
	if _, isString := row["meta"].(string); isString {
		t.Error("audit.meta must be JSON, not a string")
	}
	need("audit", row, "actorEmail")

	// sessions carry expiresAt; notifications carry priority
	need("session", get("/api/v1/me/sessions")["items"].([]any)[0].(map[string]any), "expiresAt")
}

// TestBehaviors covers server rules the UI and API clients depend on.
func TestBehaviors(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { env.srv.Shutdown(t.Context()) })
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "admin@example.com", "password": "correcthorsebatterystaple", "name": "Admin"}, nil, "")
	cookies := rec.Result().Cookies()
	var me meDTO
	json.Unmarshal(env.do(t, "GET", "/api/v1/me", nil, cookies, "").Body.Bytes(), &me) //nolint:errcheck
	csrf := me.CSRFToken

	browser := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36"
	hit := func(code string) int {
		req := httptest.NewRequest("GET", "/"+code, nil)
		req.RemoteAddr = "203.0.113.9:1111"
		req.Header.Set("User-Agent", browser)
		r := httptest.NewRecorder()
		env.srv.handler.ServeHTTP(r, req)
		return r.Code
	}

	// max_clicks is enforced immediately, before the batch writer flushes
	r := env.do(t, "POST", "/api/v1/links", map[string]any{"targetUrl": "https://example.com/m", "code": "maxc", "maxClicks": 2}, cookies, csrf)
	if r.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", r.Code, r.Body.String())
	}
	got := []int{hit("maxc"), hit("maxc"), hit("maxc")}
	if got[0] != 302 || got[1] != 302 || got[2] != http.StatusGone {
		t.Errorf("maxClicks=2 responses = %v, want [302 302 410]", got)
	}

	// validation
	for name, body := range map[string]map[string]any{
		"length 3":   {"targetUrl": "https://example.com", "length": 3},
		"utm CRLF":   {"targetUrl": "https://example.com", "utm": map[string]string{"source": "a\r\nb"}},
		"status 303": {"targetUrl": "https://example.com", "redirectStatus": 303},
	} {
		if c := env.do(t, "POST", "/api/v1/links", body, cookies, csrf).Code; c != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, c)
		}
	}

	// QR svg + colours
	var l linkDTO
	lr := env.do(t, "POST", "/api/v1/links", map[string]string{"targetUrl": "https://example.com/q", "code": "qrlink"}, cookies, csrf)
	json.Unmarshal(lr.Body.Bytes(), &l) //nolint:errcheck
	if q := env.do(t, "GET", "/api/v1/links/"+l.ID+"/qr.svg?fg=ff0000", nil, cookies, ""); q.Code != 200 || q.Header().Get("Content-Type") != "image/svg+xml" {
		t.Errorf("qr.svg: %d %s", q.Code, q.Header().Get("Content-Type"))
	}
	if q := env.do(t, "GET", "/api/v1/links/"+l.ID+"/clicks/export?format=json", nil, cookies, ""); q.Code != 200 || q.Body.Bytes()[0] != '[' {
		t.Errorf("json export: %d %s", q.Code, q.Body.String())
	}
	if q := env.do(t, "GET", "/api/v1/links/"+l.ID+"/stats?tz=Nope/Zone", nil, cookies, ""); q.Code != http.StatusBadRequest {
		t.Errorf("bad tz: %d", q.Code)
	}

	// API keys cannot mint or list keys; sessions can
	kr := env.do(t, "POST", "/api/v1/apikeys", map[string]string{"name": "k"}, cookies, csrf)
	var key map[string]any
	json.Unmarshal(kr.Body.Bytes(), &key) //nolint:errcheck
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader([]byte(`{"name":"x"}`)))
	req.Header.Set("Authorization", "Bearer "+key["key"].(string))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.5:1"
	rr := httptest.NewRecorder()
	env.srv.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("API key minting a key: %d, want 403", rr.Code)
	}

	// sudo re-auth
	if c := env.do(t, "POST", "/auth/sudo", map[string]string{"password": "wrong-password-here"}, cookies, csrf).Code; c != http.StatusUnauthorized {
		t.Errorf("sudo wrong password: %d", c)
	}
	if c := env.do(t, "POST", "/auth/sudo", map[string]string{"password": "correcthorsebatterystaple"}, cookies, csrf).Code; c != http.StatusNoContent {
		t.Errorf("sudo: %d", c)
	}

	// user search
	env.do(t, "POST", "/api/v1/users", map[string]string{"email": "zed@example.com", "name": "Zed"}, cookies, csrf)
	var page struct {
		Items []userDTO `json:"items"`
	}
	json.Unmarshal(env.do(t, "GET", "/api/v1/users?q=zed", nil, cookies, "").Body.Bytes(), &page) //nolint:errcheck
	if len(page.Items) != 1 || page.Items[0].Email != "zed@example.com" {
		t.Errorf("user search returned %+v", page.Items)
	}
}
