package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/store"
	"shortr/internal/validate"
)

const oidcCookieName = "shortr_oidc"

type oidcState struct {
	State      string `json:"state"`
	Nonce      string `json:"nonce"`
	Verifier   string `json:"verifier"`
	Next       string `json:"next"`
	LinkUserID string `json:"linkUserId,omitempty"`
}

func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		respondError(w, r, NewAPIError(http.StatusNotFound, "NOT_FOUND", "SSO is not enabled"))
		return
	}
	state, _, err := auth.NewOpaqueToken(16)
	if err != nil {
		respondError(w, r, err)
		return
	}
	nonce, _, _ := auth.NewOpaqueToken(16)
	verifier, challenge, err := auth.PKCE()
	if err != nil {
		respondError(w, r, err)
		return
	}
	st := oidcState{State: state, Nonce: nonce, Verifier: verifier, Next: validate.NextPath(r.URL.Query().Get("next"))}

	if r.URL.Query().Get("link") == "1" {
		u := userFromContext(r.Context())
		if u == nil {
			respondError(w, r, ErrUnauthenticated)
			return
		}
		if !s.hasSudo(r) {
			respondError(w, r, ErrSudoRequired)
			return
		}
		st.LinkUserID = u.ID
	}

	payload, _ := json.Marshal(st)
	token := auth.SealValue(s.cfg.SecretKey, string(payload), 10*time.Minute)
	http.SetCookie(w, &http.Cookie{Name: oidcCookieName, Value: token, Path: "/auth/oidc", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: 600})

	authURL, err := s.oidc.AuthCodeURL(r.Context(), state, nonce, challenge)
	if err != nil {
		s.renderPublic(w, http.StatusServiceUnavailable, "not_found", map[string]any{})
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		respondError(w, r, NewAPIError(http.StatusNotFound, "NOT_FOUND", "SSO is not enabled"))
		return
	}
	c, err := r.Cookie(oidcCookieName)
	if err != nil || c.Value == "" {
		s.oidcError(w, r, "OIDC_STATE_MISSING", "Your sign-in session expired or cookies are blocked. Please try again.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oidcCookieName, Value: "", Path: "/auth/oidc", MaxAge: -1})

	payload, err := auth.OpenValue(s.cfg.SecretKey, c.Value)
	if err != nil {
		s.oidcError(w, r, "OIDC_STATE_MISSING", "Your sign-in session expired. Please try again.")
		return
	}
	var st oidcState
	if err := json.Unmarshal([]byte(payload), &st); err != nil {
		s.oidcError(w, r, "OIDC_STATE_MISSING", "Invalid sign-in session.")
		return
	}

	q := r.URL.Query()
	if errCode := q.Get("error"); errCode != "" {
		s.oidcError(w, r, "OIDC_CANCELLED", "Sign-in was cancelled.")
		return
	}
	if q.Get("state") != st.State {
		s.oidcError(w, r, "OIDC_STATE_MISMATCH", "Sign-in state mismatch. Please try again.")
		return
	}
	code := q.Get("code")
	if code == "" {
		s.oidcError(w, r, "OIDC_STATE_MISSING", "Missing authorization code.")
		return
	}

	claims, err := s.oidc.Exchange(r.Context(), code, st.Verifier, st.Nonce, s.cfg.OIDCGroupsClaim)
	if err != nil {
		switch err {
		case auth.ErrEmailUnverified:
			s.oidcError(w, r, "OIDC_EMAIL_UNVERIFIED", "Your identity provider has not verified your email address.")
		case auth.ErrDomainNotAllowed:
			s.oidcError(w, r, "OIDC_DOMAIN_NOT_ALLOWED", "Your email domain is not allowed to sign in.")
		default:
			s.log.Warn("oidc exchange failed", "error", err)
			s.oidcError(w, r, "OIDC_EXCHANGE_FAILED", "Sign-in with your identity provider failed. Please try again.")
		}
		return
	}

	rawClaims, _ := json.Marshal(claims.Raw)
	if len(rawClaims) > 32*1024 {
		rawClaims = []byte("{}")
	}

	if st.LinkUserID != "" {
		s.finishOIDCLink(w, r, st, claims, string(rawClaims))
		return
	}
	s.finishOIDCLogin(w, r, st, claims, string(rawClaims))
}

func (s *Server) finishOIDCLink(w http.ResponseWriter, r *http.Request, st oidcState, claims *auth.Claims, rawClaims string) {
	existing, err := s.store.GetOIDCIdentity(r.Context(), s.cfg.OIDCIssuer, claims.Subject)
	if err == nil && existing.UserID != st.LinkUserID {
		s.oidcError(w, r, "OIDC_ALREADY_LINKED", "This SSO identity is already linked to a different account.")
		return
	}
	if err == nil {
		_ = s.store.UpdateOIDCIdentity(r.Context(), existing, time.Now())
	} else {
		id := &store.OIDCIdentity{UserID: st.LinkUserID, Issuer: s.cfg.OIDCIssuer, Subject: claims.Subject, Email: claims.Email, Name: claims.Name, RawClaims: rawClaims}
		if err := s.store.CreateOIDCIdentity(r.Context(), id); err != nil {
			respondError(w, r, err)
			return
		}
		s.audit(r, st.LinkUserID, "oidc.link", "user", st.LinkUserID, map[string]any{"issuer": s.cfg.OIDCIssuer})
	}
	http.Redirect(w, r, validate.NextPath("/app/settings/security"), http.StatusFound)
}

func (s *Server) finishOIDCLogin(w http.ResponseWriter, r *http.Request, st oidcState, claims *auth.Claims, rawClaims string) {
	ctx := r.Context()

	identity, err := s.store.GetOIDCIdentity(ctx, s.cfg.OIDCIssuer, claims.Subject)
	var u *store.User

	switch {
	case err == nil:
		u, err = s.store.GetUserByID(ctx, identity.UserID)
		if err != nil {
			s.oidcError(w, r, "OIDC_NO_ACCOUNT", "Your linked account no longer exists.")
			return
		}
		_ = s.store.UpdateOIDCIdentity(ctx, identity, time.Now())

	case claims.Email != "" && s.cfg.OIDCAutoLinkByEmail:
		existingUser, uerr := s.store.GetUserByEmail(ctx, claims.Email)
		if uerr == nil && existingUser.EmailVerified {
			u = existingUser
			id := &store.OIDCIdentity{UserID: u.ID, Issuer: s.cfg.OIDCIssuer, Subject: claims.Subject, Email: claims.Email, Name: claims.Name, RawClaims: rawClaims}
			if err := s.store.CreateOIDCIdentity(ctx, id); err != nil {
				respondError(w, r, err)
				return
			}
			s.audit(r, u.ID, "oidc.autolink", "user", u.ID, map[string]any{"issuer": s.cfg.OIDCIssuer})
			break
		}
		fallthrough

	default:
		if claims.Email != "" {
			if _, uerr := s.store.GetUserByEmail(ctx, claims.Email); uerr == nil {
				s.oidcError(w, r, "OIDC_ACCOUNT_EXISTS", "An account with this email already exists. Sign in with your password, then link SSO from Settings.")
				return
			}
		}
		if !s.cfg.OIDCAutoCreate {
			s.oidcError(w, r, "OIDC_NO_ACCOUNT", "No account found for you. Ask an admin to invite you.")
			return
		}
		role := "user"
		roleLocked := false
		if s.cfg.OIDCAdminGroup != "" && containsStr(claims.Groups, s.cfg.OIDCAdminGroup) {
			role = "admin"
		}
		now := time.Now()
		u = &store.User{Email: claims.Email, Name: claims.Name, Role: role, Status: "active", EmailVerified: claims.EmailVerified, RoleLocked: roleLocked, LastLoginAt: &now}
		if u.Email == "" {
			// no email at all: fabricate a stable placeholder so the account
			// is still addressable; admin can set a real email later.
			u.Email = "oidc-" + claims.Subject + "@" + hostFromIssuer(s.cfg.OIDCIssuer)
		}
		if err := s.store.CreateUser(ctx, u); err != nil {
			if strings.Contains(err.Error(), "conflict") {
				s.oidcError(w, r, "OIDC_ACCOUNT_EXISTS", "An account with this email already exists.")
				return
			}
			respondError(w, r, err)
			return
		}
		id := &store.OIDCIdentity{UserID: u.ID, Issuer: s.cfg.OIDCIssuer, Subject: claims.Subject, Email: claims.Email, Name: claims.Name, RawClaims: rawClaims}
		if err := s.store.CreateOIDCIdentity(ctx, id); err != nil {
			respondError(w, r, err)
			return
		}
		s.audit(r, u.ID, "user.create.oidc", "user", u.ID, nil)
		if s.notifier != nil {
			s.notifier.NotifyAdmins(ctx, "user.registered", "New SSO user", u.Email+" signed in via SSO for the first time.", map[string]any{"user_id": u.ID})
		}
	}

	if !u.IsActive() {
		s.oidcError(w, r, "USER_DISABLED", "This account has been disabled.")
		return
	}

	// group -> role sync on every login, unless a local admin locked the role
	if !u.RoleLocked && s.cfg.OIDCAdminGroup != "" {
		wantAdmin := containsStr(claims.Groups, s.cfg.OIDCAdminGroup)
		newRole := "user"
		if wantAdmin {
			newRole = "admin"
		}
		if newRole != u.Role {
			u.Role = newRole
			_ = s.store.UpdateUser(ctx, u)
		}
	}

	now := time.Now()
	u.LastLoginAt = &now
	_ = s.store.UpdateUser(ctx, u)
	s.audit(r, u.ID, "user.login.oidc", "user", u.ID, nil)
	s.startSession(w, r, u)
	http.Redirect(w, r, st.Next, http.StatusFound)
}

func (s *Server) oidcError(w http.ResponseWriter, r *http.Request, code, message string) {
	http.Redirect(w, r, "/app/login?error="+code+"&message="+url.QueryEscape(message), http.StatusFound)
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func hostFromIssuer(issuer string) string {
	h := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")
	if i := strings.IndexByte(h, '/'); i >= 0 {
		h = h[:i]
	}
	if h == "" {
		return "sso.local"
	}
	return h
}

// hasSudo reports whether the current session was established (or last
// re-authenticated) within the sudo window.
func (s *Server) hasSudo(r *http.Request) bool {
	sess := sessionFromContext(r.Context())
	if sess == nil {
		return false
	}
	if time.Since(sess.CreatedAt) < 10*time.Minute {
		return true
	}
	s.sudoMu.Lock()
	defer s.sudoMu.Unlock()
	return time.Now().Before(s.sudoUntil[sess.ID])
}
