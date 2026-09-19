package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPToolsListAndCall(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, "POST", "/auth/setup", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
	cookies := rec.Result().Cookies()
	rec = env.do(t, "GET", "/api/v1/me", nil, cookies, "")
	var me meDTO
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck

	// disabled by default
	rec = env.do(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, cookies, me.CSRFToken)
	var resp mcpResponse
	json.Unmarshal(rec.Body.Bytes(), &resp) //nolint:errcheck
	if resp.Error == nil {
		t.Fatal("expected error while MCP is disabled")
	}

	// enable it
	rec = env.do(t, "PUT", "/api/v1/settings", map[string]any{"mcpEnabled": true}, cookies, me.CSRFToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable mcp: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// create an API key scoped to links:read only
	rec = env.do(t, "POST", "/api/v1/apikeys", map[string]any{"name": "agent", "scopes": []string{"links:read"}}, cookies, me.CSRFToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create api key: %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created) //nolint:errcheck
	rawKey, _ := created["key"].(string)
	if rawKey == "" {
		t.Fatal("expected raw key")
	}

	mcpDo := func(method string, params any) mcpResponse {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.5:1"
		rr := httptest.NewRecorder()
		env.srv.handler.ServeHTTP(rr, req)
		var out mcpResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode mcp response: %v body=%s", err, rr.Body.String())
		}
		return out
	}

	// tools/list works with any authenticated key
	out := mcpDo("tools/list", nil)
	if out.Error != nil {
		t.Fatalf("tools/list error: %+v", out.Error)
	}

	// calling a links:write tool with a links:read-only key -> tool-level error
	out = mcpDo("tools/call", map[string]any{"name": "create_short_link", "arguments": map[string]any{"targetUrl": "https://example.com"}})
	if out.Error != nil {
		t.Fatalf("unexpected protocol error: %+v", out.Error)
	}
	res, _ := json.Marshal(out.Result)
	var scopeErrRes mcpToolResult
	json.Unmarshal(res, &scopeErrRes) //nolint:errcheck
	if !scopeErrRes.IsError {
		t.Fatalf("expected scope error, got: %s", res)
	}

	// list_links works with the links:read scope
	out = mcpDo("tools/call", map[string]any{"name": "list_links", "arguments": map[string]any{}})
	if out.Error != nil {
		t.Fatalf("list_links error: %+v", out.Error)
	}
	res, _ = json.Marshal(out.Result)
	var listRes mcpToolResult
	json.Unmarshal(res, &listRes) //nolint:errcheck
	if listRes.IsError {
		t.Fatalf("unexpected tool error: %s", res)
	}
}
