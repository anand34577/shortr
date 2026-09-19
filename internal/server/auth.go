package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/notify"
	"shortr/internal/store"
	"shortr/internal/validate"
)

type authStatusResp struct {
	SetupRequired   bool   `json:"setup_required"`
	OIDCEnabled     bool   `json:"oidc_enabled"`
	OIDCReady       bool   `json:"oidc_ready"`
	OIDCDisplayName string `json:"oidc_display_name"`
	LocalLogin      bool   `json:"local_login"`
	Registration    string `json:"registration"`
	SiteName        string `json:"site_name"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.CountUsers(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	resp := authStatusResp{
		SetupRequired: n == 0, OIDCEnabled: s.cfg.OIDCEnabled, OIDCDisplayName: s.cfg.OIDCDisplayName,
		LocalLogin: s.cfg.OIDCLocalLogin || !s.cfg.OIDCEnabled, Registration: s.cfg.Registration, SiteName: siteName,
	}
	if s.oidc != nil {
		resp.OIDCReady = s.oidc.Ready(r.Context())
	}
	respondJSON(w, http.StatusOK, resp)
}

type setupReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req setupReq
	if err := decodeJSON(w, r, 4096, &req); err != nil {
		respondError(w, r, err)
		return
	}
	n, err := s.store.CountUsers(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}
	if n > 0 {
		respondError(w, r, ErrSetupDone)
		return
	}

	verrs := validate.Errors{}
	email, err := validate.Email(req.Email)
	if err != nil {
		verrs.Add("email", err.Error())
	}
	if !validate.StrLen(req.Password, 10, 128) {
		verrs.Add("password", "must be 10-128 characters")
	}
	if verrs.HasAny() {
		respondError(w, r, ValidationFailed(verrs))
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, r, err)
		return
	}
	now := time.Now()
	u := &store.User{Email: email, Name: strings.TrimSpace(req.Name), PasswordHash: &hash, Role: "admin", Status: "active", EmailVerified: true, RoleLocked: true, PasswordChangedAt: &now}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "user.setup", "user", u.ID, nil)
	s.startSession(w, r, u)
	respondJSON(w, http.StatusCreated, toUserDTO(u))
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Next     string `json:"next"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(w, r, 2048, &req); err != nil {
		respondError(w, r, err)
		return
	}
	ip := clientIPFromContext(r.Context())
	lockKey := "login:" + strings.ToLower(req.Email)
	ipKey := "login-ip:" + ipStrOrEmpty(ip)
	if !s.rlAuth.Allow(lockKey) || !s.rlAuth.Allow(ipKey) {
		respondError(w, r, ErrLockedOut)
		return
	}

	email, err := validate.Email(req.Email)
	if err != nil {
		auth.VerifyAgainstDummy(req.Password)
		respondError(w, r, NewAPIError(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid email or password"))
		return
	}
	u, err := s.store.GetUserByEmail(r.Context(), email)
	if err != nil || u.PasswordHash == nil {
		auth.VerifyAgainstDummy(req.Password)
		respondError(w, r, NewAPIError(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid email or password"))
		return
	}
	ok, needsRehash, err := auth.VerifyPassword(req.Password, *u.PasswordHash)
	if err != nil || !ok {
		respondError(w, r, NewAPIError(http.StatusUnauthorized, "UNAUTHENTICATED", "invalid email or password"))
		return
	}
	if !u.IsActive() {
		respondError(w, r, ErrUserDisabled)
		return
	}
	if needsRehash {
		if h, err := auth.HashPassword(req.Password); err == nil {
			u.PasswordHash = &h
			_ = s.store.UpdateUser(r.Context(), u)
		}
	}
	now := time.Now()
	u.LastLoginAt = &now
	_ = s.store.UpdateUser(r.Context(), u)

	s.audit(r, u.ID, "user.login", "user", u.ID, nil)
	s.startSession(w, r, u)
	respondJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess := sessionFromContext(r.Context()); sess != nil {
		_ = s.store.DeleteSession(r.Context(), sess.ID)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Registration != "open" {
		respondError(w, r, ErrForbidden)
		return
	}
	var req registerReq
	if err := decodeJSON(w, r, 4096, &req); err != nil {
		respondError(w, r, err)
		return
	}
	verrs := validate.Errors{}
	email, err := validate.Email(req.Email)
	if err != nil {
		verrs.Add("email", err.Error())
	}
	if !validate.StrLen(req.Password, 10, 128) {
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
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, r, err)
		return
	}
	now := time.Now()
	u := &store.User{Email: email, Name: strings.TrimSpace(req.Name), PasswordHash: &hash, Role: "user", Status: "active", PasswordChangedAt: &now}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		respondError(w, r, err)
		return
	}
	s.audit(r, u.ID, "user.register", "user", u.ID, nil)
	if s.notifier != nil {
		s.notifier.NotifyAdmins(r.Context(), notify.KindUserRegistered, "New user registered", u.Email+" just created an account.", map[string]any{"user_id": u.ID})
	}
	s.startSession(w, r, u)
	respondJSON(w, http.StatusCreated, toUserDTO(u))
}

// --- session helpers -----------------------------------------------------

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u *store.User) {
	raw, hash, err := auth.NewOpaqueToken(32)
	if err != nil {
		respondError(w, r, err)
		return
	}
	csrfRaw, _, _ := auth.NewOpaqueToken(24)
	ip := clientIPFromContext(r.Context())
	sess := &store.Session{
		ID: hash, UserID: u.ID, CSRFToken: csrfRaw, IP: ipStrOrEmpty(ip), UserAgent: truncateStr(r.UserAgent(), 300),
		ExpiresAt: time.Now().Add(s.cfg.SessionTTL),
	}
	if err := s.store.CreateSession(r.Context(), sess); err != nil {
		respondError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: raw, Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, Expires: sess.ExpiresAt,
	})
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *Server) audit(r *http.Request, actorUserID, action, targetType, targetID string, meta map[string]any) {
	ip := clientIPFromContext(r.Context())
	metaJSON := "{}"
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			metaJSON = string(b)
		}
	}
	entry := &store.AuditEntry{ActorUserID: actorUserID, ActorIP: ipStrOrEmpty(ip), Action: action, TargetType: targetType, TargetID: targetID, Meta: metaJSON}
	if err := s.store.AddAudit(context.Background(), entry); err != nil {
		s.log.Warn("audit log write failed", "error", err)
	}
}
