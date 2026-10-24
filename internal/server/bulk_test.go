package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func adminSession(t *testing.T, env *testEnv) ([]*http.Cookie, string, string) {
	t.Helper()
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "admin@example.com", "password": "correcthorsebatterystaple", "name": "Admin"}, nil, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	var me meDTO
	json.Unmarshal(env.do(t, "GET", "/api/v1/me", nil, cookies, "").Body.Bytes(), &me) //nolint:errcheck
	return cookies, me.CSRFToken, me.ID
}

// Bulk create used to drop everything but url, code and title.
func TestBulkCreateKeepsAllFields(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { env.srv.Shutdown(t.Context()) })
	cookies, csrf, _ := adminSession(t, env)

	body := map[string]any{"items": []map[string]any{
		{"action": "create", "input": map[string]any{"targetUrl": "https://example.com/a", "tags": []string{"import"}, "maxClicks": 5}},
		{"action": "create", "input": map[string]any{"targetUrl": "ftp://nope"}},
	}}
	rec := env.do(t, "POST", "/api/v1/links/bulk", body, cookies, csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk: %d %s", rec.Code, rec.Body.String())
	}
	var res struct{ Items []bulkResult }
	json.Unmarshal(rec.Body.Bytes(), &res) //nolint:errcheck
	if len(res.Items) != 2 || !res.Items[0].OK || res.Items[0].ShortURL == "" || res.Items[1].OK {
		t.Fatalf("unexpected results: %+v", res.Items)
	}
	if res.Items[1].Error != "target_url must use http or https" {
		t.Fatalf("error message: %q", res.Items[1].Error)
	}
	l, err := env.st.GetLinkByID(t.Context(), res.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tags) != 1 || l.MaxClicks == nil || *l.MaxClicks != 5 {
		t.Fatalf("fields dropped: tags=%v maxClicks=%v", l.Tags, l.MaxClicks)
	}
}

func TestAdminPatchUserRejectsUnknownRole(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { env.srv.Shutdown(t.Context()) })
	cookies, csrf, id := adminSession(t, env)
	for _, b := range []map[string]any{{"role": "root"}, {"status": "paused"}, {"maxLinks": -1}} {
		if rec := env.do(t, "PATCH", "/api/v1/users/"+id, b, cookies, csrf); rec.Code != http.StatusBadRequest {
			t.Fatalf("%v: got %d %s", b, rec.Code, rec.Body.String())
		}
	}
}
