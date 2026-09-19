package server

import (
	"encoding/json"
	"net/http"
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
	resp := map[string]any{"user": toUserDTO(u)}
	if generated {
		resp["generated_password"] = pw
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
	users, next, err := s.store.ListUsers(r.Context(), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toUserDTO(u))
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
	MaxLinks **int   `json:"max_links"`
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

// --- settings ----------------------------------------------------------

var adminSettableKeys = map[string]bool{
	"site_name": true, "blocked_domains": true, "registration": true, "default_redirect_status": true,
	"count_bots": true, "oidc_auto_create": true, "oidc_auto_link_by_email": true, "max_links_per_user": true, "fetch_titles": true,
}

func (s *Server) handleAdminGetSettings(w http.ResponseWriter, r *http.Request, actor *store.User) {
	all, err := s.store.AllSettings(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := map[string]any{}
	for k := range adminSettableKeys {
		if v, ok := all[k]; ok {
			var parsed any
			if json.Unmarshal([]byte(v), &parsed) == nil {
				out[k] = parsed
			} else {
				out[k] = v
			}
		}
	}
	respondJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminPutSettings(w http.ResponseWriter, r *http.Request, actor *store.User) {
	var req map[string]json.RawMessage
	if err := decodeJSON(w, r, 16384, &req); err != nil {
		respondError(w, r, err)
		return
	}
	for k, v := range req {
		if !adminSettableKeys[k] {
			respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "unknown setting: "+k))
			return
		}
		if err := s.store.SetSetting(r.Context(), k, string(v), actor.ID); err != nil {
			respondError(w, r, err)
			return
		}
	}
	s.audit(r, actor.ID, "settings.update", "settings", "", nil)
	w.WriteHeader(http.StatusNoContent)
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
	for _, e := range entries {
		out = append(out, toAuditDTO(e))
	}
	respondList(w, out, next)
}

// --- admin links (all users) --------------------------------------------

func (s *Server) handleAdminListLinks(w http.ResponseWriter, r *http.Request, actor *store.User) {
	q := r.URL.Query()
	f := store.LinkFilter{Query: q.Get("q"), Status: q.Get("status"), Cursor: q.Get("cursor")}
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

func (s *Server) handleAdminSystem(w http.ResponseWriter, r *http.Request, actor *store.User) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	dbOK := s.store.Ping(r.Context()) == nil
	respondJSON(w, http.StatusOK, map[string]any{
		"version": s.version, "uptime_seconds": int(time.Since(s.startTime).Seconds()),
		"db_driver": s.cfg.DBDriver, "db_ok": dbOK,
		"cache_entries": s.links.Cache().Len(),
		"goroutines":    runtime.NumGoroutine(), "memory_alloc_bytes": ms.Alloc,
		"oidc_enabled": s.cfg.OIDCEnabled, "smtp_enabled": s.cfg.SMTPEnabled, "gotify_enabled": s.cfg.GotifyEnabled,
	})
}
