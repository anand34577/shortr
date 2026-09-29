package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"shortr/internal/auth"
	"shortr/internal/store"
)

const sessionCookieName = "shortr_session"

// authenticate is not a middleware itself — it's called by every handler
// (via s.currentUser(r)) that cares about identity, because some routes
// (the redirect hot path, public auth endpoints) must never pay for it.
// Handlers that require auth call s.requireUser / s.requireAdmin.

type identity struct {
	User    *store.User
	Session *store.Session // nil if authenticated via API key
	APIKey  *store.APIKey  // nil if authenticated via session cookie
}

// resolveIdentity checks the session cookie, then the Authorization header,
// returning (nil, nil) if unauthenticated rather than an error — callers
// decide whether auth is required.
func (s *Server) resolveIdentity(r *http.Request) (*identity, error) {
	ctx := r.Context()

	// The public plane in split mode is token-only: a stolen or cross-site
	// cookie is worthless there.
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" && planeOf(r) != planePublic {
		hash := auth.HashToken(c.Value)
		sess, err := s.store.GetSession(ctx, hash)
		if err == nil {
			if sess.ExpiresAt.Before(time.Now()) {
				return nil, nil
			}
			u, err := s.store.GetUserByID(ctx, sess.UserID)
			if err == nil && u.IsActive() {
				if time.Since(sess.LastSeenAt) > 5*time.Minute {
					_ = s.store.TouchSession(ctx, sess.ID, time.Now())
				}
				return &identity{User: u, Session: sess}, nil
			}
		}
	}

	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		raw := strings.TrimPrefix(h, "Bearer ")
		if strings.HasPrefix(raw, "sk_") {
			hash := auth.HashToken(raw)
			key, err := s.store.GetAPIKeyByHash(ctx, hash)
			if err == nil {
				if key.RevokedAt != nil || (key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now())) {
					return nil, nil
				}
				u, err := s.store.GetUserByID(ctx, key.UserID)
				if err == nil && u.IsActive() {
					if key.LastUsedAt == nil || time.Since(*key.LastUsedAt) > time.Minute {
						_ = s.store.TouchAPIKey(ctx, key.ID, time.Now())
					}
					return &identity{User: u, APIKey: key}, nil
				}
			}
		} else if s.oidc != nil && len(s.cfg.OIDCAPIAudiences) > 0 && strings.Count(raw, ".") == 2 {
			return s.resolveOIDCBearer(r, raw), nil
		}
	}

	return nil, nil
}

// oidcTokenScopes: what an IdP access token may do. Deliberately no admin:*
// and, because it rides the APIKey slot, no session-only routes (keys,
// password, sessions) — the same limits as a scoped API key.
var oidcTokenScopes = []string{"links:read", "links:write", "stats:read"}

// resolveOIDCBearer maps an IdP access token (e.g. from the Android app's
// Keycloak login) to an existing local user through an already-linked SSO
// identity. It never creates accounts: sign in to the web UI once first.
func (s *Server) resolveOIDCBearer(r *http.Request, raw string) *identity {
	sub, err := s.oidc.VerifyAccessToken(r.Context(), raw, s.cfg.OIDCAPIAudiences)
	if err != nil {
		s.log.Debug("oidc bearer rejected", "error", err)
		return nil
	}
	oid, err := s.store.GetOIDCIdentity(r.Context(), s.cfg.OIDCIssuer, sub)
	if err != nil {
		return nil
	}
	u, err := s.store.GetUserByID(r.Context(), oid.UserID)
	if err != nil || !u.IsActive() {
		return nil
	}
	return &identity{User: u, APIKey: &store.APIKey{ID: "oidc:" + sub, UserID: u.ID, Name: "SSO access token", Scopes: oidcTokenScopes}}
}

// withIdentityMiddleware resolves identity once per request and stashes it
// in context so downstream handlers/middleware don't repeat the DB lookup.
func (s *Server) withIdentityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !needsIdentity(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		id, err := s.resolveIdentity(r)
		if err == nil && id != nil {
			ctx := context.WithValue(r.Context(), ctxKeyUser, id.User)
			if id.Session != nil {
				ctx = context.WithValue(ctx, ctxKeySession, id.Session)
			}
			if id.APIKey != nil {
				ctx = context.WithValue(ctx, ctxKeyAPIKey, id.APIKey)
			}
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func apiKeyFromContext(ctx context.Context) *store.APIKey {
	v, _ := ctx.Value(ctxKeyAPIKey).(*store.APIKey)
	return v
}

// requireAuth wraps an API handler that needs an authenticated user.
func (s *Server) requireAuth(h func(w http.ResponseWriter, r *http.Request, u *store.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userFromContext(r.Context())
		if u == nil {
			respondError(w, r, ErrUnauthenticated)
			return
		}
		if !mfaGateAllows(r) && s.mfaEnrollRequired(r, u) {
			respondError(w, r, ErrMFAEnrollRequired)
			return
		}
		h(w, r, u)
	}
}

// requireAdmin wraps an API handler that needs an authenticated admin.
// requireSession is requireAuth restricted to browser sessions: API keys may
// not manage keys, passwords or sessions.
func (s *Server) requireSession(h func(w http.ResponseWriter, r *http.Request, u *store.User)) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		if apiKeyFromContext(r.Context()) != nil {
			respondError(w, r, ErrForbidden)
			return
		}
		h(w, r, u)
	})
}

// requireAdmin wraps an API handler that needs an authenticated admin. When
// the caller authenticated with an API key (not a session cookie), the key
// must additionally carry the admin:* scope — otherwise a narrowly-scoped
// key (e.g. links:read) belonging to an admin user would inherit full admin
// power over every admin-only endpoint.
func (s *Server) requireAdmin(h func(w http.ResponseWriter, r *http.Request, u *store.User)) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		if !u.IsAdmin() {
			respondError(w, r, ErrForbidden)
			return
		}
		if key := apiKeyFromContext(r.Context()); key != nil && !hasScope(key.Scopes, "admin:*") {
			respondError(w, r, NewAPIError(http.StatusForbidden, "FORBIDDEN", "API key missing required scope: admin:*"))
			return
		}
		h(w, r, u)
	})
}

// requireScope additionally checks that an API-key-authenticated request
// carries the given scope; session-authenticated requests implicitly have
// every scope their role allows.
func (s *Server) requireScope(scope string, h func(w http.ResponseWriter, r *http.Request, u *store.User)) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		if key := apiKeyFromContext(r.Context()); key != nil {
			if !hasScope(key.Scopes, scope) {
				respondError(w, r, NewAPIError(http.StatusForbidden, "FORBIDDEN", "API key missing required scope: "+scope))
				return
			}
		}
		h(w, r, u)
	})
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want || s == "admin:*" {
			return true
		}
	}
	return false
}

// csrfMiddleware enforces the double-submit header + Origin check for
// session-cookie-authenticated mutating requests. API-key
// (Bearer) requests carry no cookie and are exempt.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		sess := sessionFromContext(r.Context())
		if sess == nil {
			next.ServeHTTP(w, r) // API-key or unauthenticated (handler enforces auth separately)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !s.sameOrigin(r, origin) {
			respondError(w, r, ErrCSRFFailed)
			return
		}
		token := r.Header.Get("X-CSRF-Token")
		if token == "" || !auth.ConstantTimeEqual(token, sess.CSRFToken) {
			respondError(w, r, ErrCSRFFailed)
			return
		}
		next.ServeHTTP(w, r)
	})
}
