package server

import (
	"net/http"

	"shortr/internal/store"
)

func (s *Server) routes() {
	mux := http.NewServeMux()

	// --- ops ---------------------------------------------------------
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /version", s.handleVersion)
	mux.HandleFunc("GET /api/v1/openapi.json", s.handleOpenAPI)
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})

	// --- auth ----------------------------------------------------------
	mux.HandleFunc("GET /auth/status", s.handleAuthStatus)
	mux.Handle("POST /auth/setup", s.rateLimitMiddleware(s.rlAuth, "auth", ipKeyFn)(http.HandlerFunc(s.handleSetup)))
	mux.Handle("POST /auth/login", s.rateLimitMiddleware(s.rlAuth, "auth", ipKeyFn)(http.HandlerFunc(s.handleLogin)))
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /auth/sudo", s.requireAuth(s.handleSudo))
	mux.Handle("POST /auth/register", s.rateLimitMiddleware(s.rlAuth, "auth", ipKeyFn)(http.HandlerFunc(s.handleRegister)))
	mux.Handle("GET /auth/oidc/start", s.rateLimitMiddleware(s.rlAuth, "auth", ipKeyFn)(http.HandlerFunc(s.handleOIDCStart)))
	mux.HandleFunc("GET /auth/oidc/callback", s.handleOIDCCallback)

	// --- API: me ---------------------------------------------------
	mux.HandleFunc("GET /api/v1/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("PATCH /api/v1/me", s.requireAuth(s.handlePatchMe))
	mux.HandleFunc("PUT /api/v1/me/password", s.requireSession(s.handlePutPassword))
	mux.HandleFunc("DELETE /api/v1/me", s.requireSession(s.handleDeleteMe))
	mux.HandleFunc("GET /api/v1/me/sessions", s.requireSession(s.handleListMySessions))
	mux.HandleFunc("DELETE /api/v1/me/sessions/{id}", s.requireSession(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleDeleteMySession(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/me/identities", s.requireAuth(s.handleListMyIdentities))
	mux.HandleFunc("DELETE /api/v1/me/identities/{id}", s.requireAuth(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleDeleteMyIdentity(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /auth/oidc/link", s.requireAuth(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleOIDCStart(w, r)
	}))

	// --- API: notifications -------------------------------------------
	mux.HandleFunc("GET /api/v1/notifications", s.requireAuth(s.handleListNotifications))
	mux.HandleFunc("PATCH /api/v1/notifications/{id}/read", s.requireAuth(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleMarkNotificationRead(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("POST /api/v1/notifications/read-all", s.requireAuth(s.handleMarkAllNotificationsRead))
	mux.HandleFunc("GET /api/v1/notifications/preferences", s.requireAuth(s.handleGetNotifyPrefs))
	mux.HandleFunc("PUT /api/v1/notifications/preferences", s.requireAuth(s.handlePutNotifyPrefs))

	// --- API: links ------------------------------------------------
	mux.HandleFunc("POST /api/v1/links", s.requireScope("links:write", s.handleCreateLink))
	mux.HandleFunc("GET /api/v1/links", s.requireScope("links:read", s.handleListLinks))
	mux.HandleFunc("POST /api/v1/links/bulk", s.requireScope("links:write", s.handleBulkLinks))
	mux.HandleFunc("GET /api/v1/links/check", s.requireScope("links:read", s.handleCheckCode))
	mux.HandleFunc("POST /api/v1/links/preview", s.requireScope("links:write", s.handleLinkPreview))
	mux.HandleFunc("GET /api/v1/links/{id}", s.requireScope("links:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleGetLink(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("PATCH /api/v1/links/{id}", s.requireScope("links:write", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handlePatchLink(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("DELETE /api/v1/links/{id}", s.requireScope("links:write", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleDeleteLink(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("POST /api/v1/links/{id}/restore", s.requireScope("links:write", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleRestoreLink(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/links/{id}/qr.png", s.requireScope("links:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleLinkQR(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/links/{id}/qr.svg", s.requireScope("links:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleLinkQRSVG(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/links/{id}/stats", s.requireScope("stats:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleLinkStats(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/links/{id}/clicks", s.requireScope("stats:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleListClicks(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/links/{id}/clicks/export", s.requireScope("stats:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleExportClicks(w, r, u, r.PathValue("id"))
	}))

	// --- API: stats ------------------------------------------------
	mux.HandleFunc("GET /api/v1/stats/overview", s.requireScope("stats:read", s.handleGlobalStats))
	mux.HandleFunc("GET /api/v1/stats/recent", s.requireScope("stats:read", s.handleRecentActivity))

	// --- API: tools (optional external integrations) ------------------
	mux.HandleFunc("GET /api/v1/tools/ip-lookup/{ip}", s.requireScope("stats:read", func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleIPLookup(w, r, u, r.PathValue("ip"))
	}))

	// --- MCP server (AI agent integration, optional) -------------------
	mux.HandleFunc("POST /mcp", s.requireAuth(s.handleMCP))

	// --- API: api keys -----------------------------------------------
	mux.HandleFunc("POST /api/v1/apikeys", s.requireSession(s.handleCreateAPIKey))
	mux.HandleFunc("GET /api/v1/apikeys", s.requireSession(s.handleListAPIKeys))
	mux.HandleFunc("DELETE /api/v1/apikeys/{id}", s.requireSession(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleDeleteAPIKey(w, r, u, r.PathValue("id"))
	}))

	// --- admin ---------------------------------------------------------
	mux.HandleFunc("POST /api/v1/users", s.requireAdmin(s.handleAdminCreateUser))
	mux.HandleFunc("GET /api/v1/users", s.requireAdmin(s.handleAdminListUsers))
	mux.HandleFunc("GET /api/v1/users/{id}", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminGetUser(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("PATCH /api/v1/users/{id}", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminPatchUser(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("DELETE /api/v1/users/{id}", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminDeleteUser(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("POST /api/v1/users/{id}/reset-password", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminResetPassword(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("DELETE /api/v1/users/{id}/sessions", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminDeleteUserSessions(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("DELETE /api/v1/users/{id}/identities/{iid}", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminDeleteUserIdentity(w, r, u, r.PathValue("id"), r.PathValue("iid"))
	}))
	mux.HandleFunc("GET /api/v1/settings", s.requireAdmin(s.handleAdminGetSettings))
	mux.HandleFunc("PUT /api/v1/settings", s.requireAdmin(s.handleAdminPutSettings))
	mux.HandleFunc("GET /api/v1/audit", s.requireAdmin(s.handleAdminAudit))
	mux.HandleFunc("GET /api/v1/admin/links", s.requireAdmin(s.handleAdminListLinks))
	mux.HandleFunc("POST /api/v1/admin/links/{id}/purge", s.requireAdmin(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		s.handleAdminPurgeLink(w, r, u, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/v1/admin/system", s.requireAdmin(s.handleAdminSystem))
	mux.HandleFunc("POST /api/v1/admin/backup", s.requireAdmin(s.handleAdminBackup))

	// --- SPA -------------------------------------------------------
	mux.Handle("GET /app/", s.spaHandler())
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app", http.StatusFound)
	})

	// --- redirect hot path (catch-all) --------------------------------
	redirectChain := chain(http.HandlerFunc(s.handleRedirect), func(h http.Handler) http.Handler {
		return s.rateLimitMiddleware(s.rlRedirect, "redirect", ipKeyFn)(h)
	})
	mux.Handle("/", redirectChain)

	// full middleware stack, outermost first
	s.handler = chain(mux,
		s.recoverMiddleware,
		s.requestIDMiddleware,
		s.realIPMiddleware,
		s.loggingMiddleware,
		s.securityHeadersMiddleware,
		s.corsMiddleware,
		s.withIdentityMiddleware,
		s.apiRateLimitMiddleware,
		s.csrfMiddleware,
	)
}
