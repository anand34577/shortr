package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/store"
	"shortr/internal/validate"
)

// --- users -----------------------------------------------------------

type createUserReq struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request, actor *store.User) {
	var req createUserReq
	if err := decodeJSON(w, r, 4096, &req); err != nil {
		respondError(w, r, err)
		return
	}
	verrs := validate.Errors{}
	email, err := validate.Email(req.Email)
	if err != nil {
		verrs.Add("email", err.Error())
	}
	if req.Role != "" && req.Role != "user" && req.Role != "admin" {
		verrs.Add("role", "must be user or admin")
	}
	pw := req.Password
	generated := false
	if pw == "" {
		pw = generatePassword()
		generated = true
	} else if !validate.StrLen(pw, 10, 128) {
		verrs.Add("password", "must be 10-128 characters")
	}
	if verrs.HasAny() {
		respondError(w, r, ValidationFailed(verrs))
		return
	}
	if _, err := s.store.GetUserByEmail(r.Context(), email); err == nil {
		respondError(w, r, ErrEmailTaken)
		return
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		respondError(w, r, err)
		return
	}
	role := req.Role
	if role == "" {
		role = "user"
	}
	u := &store.User{Email: email, Name: strings.TrimSpace(req.Name), PasswordHash: &hash, Role: role, Status: "active", MustChangePassword: true}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, actor.ID, "user.admin_create", "user", u.ID, nil)
	resp := struct {
		userDTO
		GeneratedPassword string `json:"generatedPassword,omitempty"`
	}{userDTO: toUserDTO(u)}
	if generated {
		resp.GeneratedPassword = pw
	}
	respondJSON(w, http.StatusCreated, resp)
}

func generatePassword() string {
	raw, _, _ := auth.NewOpaqueToken(18)
	return raw
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request, actor *store.User) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	users, next, err := s.store.ListUsers(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		d := toUserDTO(u)
		if n, err := s.store.CountLinksForUser(r.Context(), u.ID); err == nil {
			d.LinksCount = &n
		}
		out = append(out, d)
	}
	respondList(w, out, next)
}

func (s *Server) handleAdminGetUser(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return
	}
	respondJSON(w, http.StatusOK, toUserDTO(u))
}

type patchUserReq struct {
	Name     *string `json:"name"`
	Role     *string `json:"role"`
	Status   *string `json:"status"`
	MaxLinks **int   `json:"maxLinks"`
}

func (s *Server) handleAdminPatchUser(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return
	}
	var req patchUserReq
	if err := decodeJSON(w, r, 4096, &req); err != nil {
		respondError(w, r, err)
		return
	}
	if req.Name != nil {
		u.Name = strings.TrimSpace(*req.Name)
	}
	if req.Role != nil && *req.Role != u.Role {
		if u.Role == "admin" && *req.Role != "admin" {
			n, err := s.store.CountAdmins(r.Context(), u.ID)
			if err != nil {
				respondError(w, r, err)
				return
			}
			if n == 0 {
				respondError(w, r, ErrLastAdmin)
				return
			}
		}
		u.Role = *req.Role
		u.RoleLocked = true // an explicit admin edit always wins over OIDC group sync
	}
	if req.Status != nil && *req.Status != u.Status {
		if u.Status == "active" && *req.Status == "disabled" && u.IsAdmin() {
			n, err := s.store.CountAdmins(r.Context(), u.ID)
			if err != nil {
				respondError(w, r, err)
				return
			}
			if n == 0 {
				respondError(w, r, ErrLastAdmin)
				return
			}
		}
		u.Status = *req.Status
		if u.Status == "disabled" {
			_ = s.store.DeleteSessionsForUser(r.Context(), u.ID)
		}
	}
	if req.MaxLinks != nil {
		u.MaxLinks = *req.MaxLinks
	}
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, actor.ID, "user.admin_update", "user", u.ID, nil)
	respondJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *Server) handleAdminResetPassword(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return
	}
	pw := generatePassword()
	hash, err := auth.HashPassword(pw)
	if err != nil {
		respondError(w, r, err)
		return
	}
	now := time.Now()
	u.PasswordHash = &hash
	u.PasswordChangedAt = &now
	u.MustChangePassword = true
	if err := s.store.UpdateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	_ = s.store.DeleteSessionsForUser(r.Context(), u.ID)
	s.audit(r, actor.ID, "user.admin_reset_password", "user", u.ID, nil)
	respondJSON(w, http.StatusOK, map[string]any{"password": pw})
}

func (s *Server) handleAdminDeleteUserSessions(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	if err := s.store.DeleteSessionsForUser(r.Context(), id); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, actor.ID, "user.admin_revoke_sessions", "user", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminDeleteUserIdentity(w http.ResponseWriter, r *http.Request, actor *store.User, userID, identityID string) {
	identity, err := s.store.GetOIDCIdentityByID(r.Context(), identityID)
	if err != nil || identity.UserID != userID {
		respondError(w, r, ErrNotFound)
		return
	}
	if err := s.store.DeleteOIDCIdentity(r.Context(), identityID); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, actor.ID, "user.admin_unlink_oidc", "user", userID, nil)
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminDeleteUser permanently removes a user. Their sessions, API
// keys, and OIDC identities cascade-delete with them (FK ON DELETE
// CASCADE); their links are kept but orphaned (user_id -> NULL via ON
// DELETE SET NULL), matching the "keep" behavior of a self-service account
// deletion (an admin
// deleting a user always keeps links, since there's no one left to ask).
func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	u, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return
	}
	if u.IsAdmin() {
		n, err := s.store.CountAdmins(r.Context(), u.ID)
		if err != nil {
			respondError(w, r, err)
			return
		}
		if n == 0 {
			respondError(w, r, ErrLastAdmin)
			return
		}
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, actor.ID, "user.admin_delete", "user", id, map[string]any{"email": u.Email})
	w.WriteHeader(http.StatusNoContent)
}

// --- settings ----------------------------------------------------------

// adminSettableKeys maps the wire (camelCase, matching web/src/lib/types.ts
// AdminSettings) field name to its internal settings-table key. Runtime
// editable; everything else in AdminSettings (baseUrl,
// ipMode, oidcEnabled, clickRetentionDays) is env-var-driven and
// intentionally read-only here — see handleAdminGetSettings.
var adminSettableKeys = map[string]string{
	"siteName": "site_name", "blockedDomains": "blocked_domains", "registration": "registration",
	"defaultRedirectStatus": "default_redirect_status", "countBots": "count_bots",
	"oidcAutoCreate": "oidc_auto_create", "oidcAutoLinkByEmail": "oidc_auto_link_by_email",
	"maxLinksPerUser": "max_links_per_user", "fetchTitles": "fetch_titles",
	"ipLocationEnabled": "iplocation_enabled", "ipLocationBaseUrl": "iplocation_base_url",
	"mcpEnabled": "mcp_enabled",
}

func (s *Server) handleAdminGetSettings(w http.ResponseWriter, r *http.Request, actor *store.User) {
	all, err := s.store.AllSettings(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := map[string]any{
		// env-driven, read-only from this endpoint's perspective
		"baseUrl": s.cfg.BaseURL, "ipMode": s.cfg.IPMode, "oidcEnabled": s.cfg.OIDCEnabled,
		"clickRetentionDays": s.cfg.ClickRetentionDays,
		// defaults for anything never explicitly set via PUT
		"siteName": siteName, "registration": s.cfg.Registration, "defaultRedirectStatus": s.cfg.DefaultRedirectCode,
		"countBots": s.cfg.CountBots, "oidcAutoCreate": s.cfg.OIDCAutoCreate, "oidcAutoLinkByEmail": s.cfg.OIDCAutoLinkByEmail,
		"maxLinksPerUser": s.cfg.MaxLinksPerUser, "fetchTitles": s.cfg.FetchTitles, "blockedDomains": s.cfg.BlockedDomains,
		"ipLocationEnabled": false, "ipLocationBaseUrl": "", "mcpEnabled": false,
	}
	for wireKey, storeKey := range adminSettableKeys {
		if v, ok := all[storeKey]; ok {
			var parsed any
			if json.Unmarshal([]byte(v), &parsed) == nil {
				out[wireKey] = parsed
			}
		}
	}
	if bd, ok := out["blockedDomains"]; !ok || bd == nil {
		out["blockedDomains"] = []string{}
	} else if l, ok := bd.([]string); ok && l == nil {
		out["blockedDomains"] = []string{}
	}
	respondJSON(w, http.StatusOK, out)
}

// ipLocationSettings reads the runtime-configurable IP location checker
// toggle + base URL. Absent/malformed values default to disabled.
func (s *Server) ipLocationSettings(ctx context.Context) (enabled bool, baseURL string) {
	if v, ok, _ := s.store.GetSetting(ctx, "iplocation_enabled"); ok {
		_ = json.Unmarshal([]byte(v), &enabled)
	}
	if v, ok, _ := s.store.GetSetting(ctx, "iplocation_base_url"); ok {
		_ = json.Unmarshal([]byte(v), &baseURL)
	}
	return enabled, baseURL
}

// mcpEnabled reports whether the MCP server endpoint is turned on.
func (s *Server) mcpEnabled(ctx context.Context) bool {
	var enabled bool
	if v, ok, _ := s.store.GetSetting(ctx, "mcp_enabled"); ok {
		_ = json.Unmarshal([]byte(v), &enabled)
	}
	return enabled
}

func (s *Server) handleAdminPutSettings(w http.ResponseWriter, r *http.Request, actor *store.User) {
	var req map[string]json.RawMessage
	if err := decodeJSON(w, r, 16384, &req); err != nil {
		respondError(w, r, err)
		return
	}
	for k, v := range req {
		storeKey, ok := adminSettableKeys[k]
		if !ok {
			continue // silently ignore read-only/unknown fields (e.g. baseUrl) instead of failing the whole save
		}
		if err := s.store.SetSetting(r.Context(), storeKey, string(v), actor.ID); err != nil {
			respondError(w, r, err)
			return
		}
	}
	s.audit(r, actor.ID, "settings.update", "settings", "", nil)
	s.handleAdminGetSettings(w, r, actor)
}

// --- audit ---------------------------------------------------------------

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request, actor *store.User) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	entries, next, err := s.store.ListAudit(r.Context(), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]auditDTO, 0, len(entries))
	emails := map[string]string{}
	for _, e := range entries {
		d := toAuditDTO(e)
		if e.ActorUserID != "" {
			em, seen := emails[e.ActorUserID]
			if !seen {
				if au, err := s.store.GetUserByID(r.Context(), e.ActorUserID); err == nil {
					em = au.Email
				}
				emails[e.ActorUserID] = em
			}
			d.ActorEmail = em
		}
		out = append(out, d)
	}
	respondList(w, out, next)
}

// --- admin links (all users) --------------------------------------------

func (s *Server) handleAdminListLinks(w http.ResponseWriter, r *http.Request, actor *store.User) {
	q := r.URL.Query()
	f := store.LinkFilter{Query: q.Get("q"), Status: q.Get("status"), Cursor: q.Get("cursor"), UserID: q.Get("user_id")}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	links, next, err := s.store.ListLinks(r.Context(), f)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]linkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, s.toLinkDTO(l))
	}
	respondList(w, out, next)
}

func (s *Server) handleAdminPurgeLink(w http.ResponseWriter, r *http.Request, actor *store.User, id string) {
	l, err := s.store.GetLinkByID(r.Context(), id)
	if err != nil {
		respondError(w, r, ErrNotFound)
		return
	}
	if err := s.store.PurgeLink(r.Context(), id); err != nil {
		respondError(w, r, err)
		return
	}
	s.links.InvalidateCache(l.Code)
	s.audit(r, actor.ID, "link.admin_purge", "link", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// --- system --------------------------------------------------------------

// handleAdminBackup triggers an immediate SQLite snapshot (VACUUM INTO) on
// demand from the admin System page, in addition to the scheduled
// SHORTR_BACKUP_INTERVAL job.
func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request, actor *store.User) {
	if s.cfg.DBDriver != "sqlite" {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "on-demand backup is only supported for the sqlite driver; use pg_dump for postgres"))
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		respondError(w, r, err)
		return
	}
	dest := filepath.Join(dir, "manual-"+time.Now().UTC().Format("20060102T150405")+".db")
	if err := s.store.BackupSQLite(r.Context(), dest); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, actor.ID, "system.backup", "system", "", map[string]any{"path": dest})
	respondJSON(w, http.StatusOK, map[string]any{"path": dest})
}

func (s *Server) handleAdminSystem(w http.ResponseWriter, r *http.Request, actor *store.User) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	dbOK := s.store.Ping(r.Context()) == nil

	hits := s.metrics.Get("shortr_cache_hits_total", nil)
	misses := s.metrics.Get("shortr_cache_misses_total", nil)
	var hitRatio float64
	if total := hits + misses; total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	var detectedProxyIP string
	if r.Header.Get("X-Forwarded-For") != "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			if remote := net.ParseIP(host); remote != nil && !ipInAny(remote, s.cfg.TrustedProxies) {
				// a forwarding header arrived from an IP we don't trust —
				// most likely SHORTR_TRUSTED_PROXIES needs this address.
				detectedProxyIP = host
			}
		}
	}

	var lastBackupAt *time.Time
	if entries, err := os.ReadDir(filepath.Join(s.cfg.DataDir, "backups")); err == nil {
		var latest time.Time
		for _, e := range entries {
			if info, err := e.Info(); err == nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
		if !latest.IsZero() {
			lastBackupAt = &latest
		}
	}

	ipLocEnabled, ipLocBaseURL := s.ipLocationSettings(r.Context())

	respondJSON(w, http.StatusOK, map[string]any{
		"version": s.version, "commit": s.commit, "uptimeSeconds": int(time.Since(s.startTime).Seconds()),
		"dbDriver": s.cfg.DBDriver, "dbOk": dbOK, "dbSizeBytes": s.store.DBSizeBytes(r.Context(), s.cfg.DBDSN),
		"cacheEntries": s.links.Cache().Len(), "cacheHitRatio": hitRatio,
		"queueDepth": s.metrics.Get("shortr_clicks_queued", nil), "droppedClicks": s.metrics.Get("shortr_clicks_dropped_total", nil),
		"goroutines": runtime.NumGoroutine(), "memoryAllocBytes": ms.Alloc,
		"oidcEnabled": s.cfg.OIDCEnabled, "smtpEnabled": s.cfg.SMTPEnabled, "gotifyEnabled": s.cfg.GotifyEnabled,
		"ipLocationEnabled": ipLocEnabled && ipLocBaseURL != "", "mcpEnabled": s.mcpEnabled(r.Context()),
		"lastBackupAt": lastBackupAt, "detectedProxyIp": detectedProxyIP,
	})
}
