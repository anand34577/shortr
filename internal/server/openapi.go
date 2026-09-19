package server

import (
	"net/http"
	"strings"
)

type apiOp struct{ method, path, summary string }

// apiOps documents the public REST surface for GET /api/v1/openapi.json.
var apiOps = []apiOp{
	{"post", "/auth/login", "Sign in with email and password"},
	{"post", "/auth/logout", "Sign out"},
	{"get", "/api/v1/me", "Current user, CSRF token and capabilities"},
	{"patch", "/api/v1/me", "Update profile"},
	{"put", "/api/v1/me/password", "Change password"},
	{"get", "/api/v1/links", "List links (q, tag, status, sort, order, cursor, limit)"},
	{"post", "/api/v1/links", "Create a link"},
	{"post", "/api/v1/links/bulk", "Bulk create, delete, enable, disable or tag links"},
	{"get", "/api/v1/links/check", "Check whether an alias is available"},
	{"post", "/api/v1/links/preview", "Fetch the title of a target URL"},
	{"get", "/api/v1/links/{id}", "Get a link"},
	{"patch", "/api/v1/links/{id}", "Update a link"},
	{"delete", "/api/v1/links/{id}", "Delete a link (restorable for 30 days)"},
	{"post", "/api/v1/links/{id}/restore", "Restore a deleted link"},
	{"get", "/api/v1/links/{id}/qr.png", "QR code image"},
	{"get", "/api/v1/links/{id}/stats", "Click statistics for a link"},
	{"get", "/api/v1/links/{id}/clicks", "Recent clicks for a link"},
	{"get", "/api/v1/links/{id}/clicks/export", "Export clicks as CSV"},
	{"get", "/api/v1/stats/overview", "Account-wide statistics"},
	{"get", "/api/v1/stats/recent", "Recent click activity"},
	{"get", "/api/v1/apikeys", "List API keys"},
	{"post", "/api/v1/apikeys", "Create an API key"},
	{"delete", "/api/v1/apikeys/{id}", "Revoke an API key"},
	{"get", "/api/v1/notifications", "List notifications"},
	{"get", "/api/v1/users", "List users (admin)"},
	{"post", "/api/v1/users", "Create a user (admin)"},
	{"get", "/api/v1/settings", "Get instance settings (admin)"},
	{"put", "/api/v1/settings", "Update instance settings (admin)"},
	{"get", "/api/v1/audit", "Audit log (admin)"},
	{"get", "/api/v1/admin/system", "System information (admin)"},
	{"get", "/api/v1/tools/ip-lookup/{ip}", "Look up geolocation/ASN info for an IP (requires the optional IP location checker to be configured)"},
	{"post", "/mcp", "Model Context Protocol (MCP) JSON-RPC endpoint for AI agent integrations (requires the MCP server to be enabled)"},
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	paths := map[string]map[string]any{}
	for _, op := range apiOps {
		if paths[op.path] == nil {
			paths[op.path] = map[string]any{}
		}
		paths[op.path][op.method] = map[string]any{
			"summary":   op.summary,
			"responses": map[string]any{"200": map[string]any{"description": "OK"}},
		}
	}
	version := s.version
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "Shortr API", "version": version},
		"paths":   paths,
	})
}
