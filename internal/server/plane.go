package server

import (
	"context"
	"net/http"
	"strings"
)

// A plane is which listener a request arrived on. In single-port mode every
// request is planeAll. With SHORTR_ADMIN_LISTEN set, the public listener
// (the one you put behind a Cloudflare Tunnel) is planePublic and serves only
// redirects plus a token-authenticated API; the UI, cookie sign-in and ops
// endpoints live on planeAdmin, which you keep on your LAN/VPN.
type plane int

const (
	planeAll plane = iota
	planePublic
	planeAdmin
)

type planeKey struct{}

func withPlane(p plane, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), planeKey{}, p)))
	})
}

func planeOf(r *http.Request) plane {
	p, _ := r.Context().Value(planeKey{}).(plane)
	return p
}

// adminOnlyAPI lists API prefixes that manage the whole instance. They are
// hidden on the public plane unless SHORTR_PUBLIC_ADMIN_API=true.
var adminOnlyAPI = []string{
	"/api/v1/users", "/api/v1/settings", "/api/v1/audit", "/api/v1/admin/",
}

// publicAllowed decides what the public plane exposes in split mode. Short
// codes can never collide with these prefixes because they are reserved in
// link.Reserved.
func (s *Server) publicAllowed(path string) bool {
	switch {
	case path == "/healthz" || path == "/readyz" || path == "/robots.txt":
		return true
	case path == "/metrics" || path == "/version":
		return false // ops data stays on the admin plane
	case path == "/app" || strings.HasPrefix(path, "/app/") || strings.HasPrefix(path, "/auth/"):
		return false // no UI and no cookie sign-in on the public side
	case path == "/mcp" || strings.HasPrefix(path, "/api/"):
		if !s.cfg.PublicAPI {
			return false
		}
		if !s.cfg.PublicAdminAPI {
			for _, p := range adminOnlyAPI {
				if strings.HasPrefix(path, p) {
					return false
				}
			}
		}
		return true
	}
	return true // redirects and link-password pages
}

// publicGate runs before everything else on the public listener, so hidden
// routes look exactly like an unknown short code (404, same page).
func (s *Server) publicGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.publicAllowed(r.URL.Path) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" {
				respondError(w, r, ErrNotFound)
			} else {
				s.renderPublic(w, http.StatusNotFound, "not_found", nil)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleRoot: the admin console where the UI lives, else the configured
// homepage, else a plain 404.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	switch {
	case s.cfg.UIEnabled && planeOf(r) != planePublic:
		http.Redirect(w, r, "/app", http.StatusFound)
	case s.cfg.RootRedirect != "":
		http.Redirect(w, r, s.cfg.RootRedirect, http.StatusFound)
	default:
		s.renderPublic(w, http.StatusNotFound, "not_found", nil)
	}
}

// cookieSecure: the admin plane may be plain HTTP on a LAN while the public
// base URL is HTTPS, so it follows SHORTR_ADMIN_URL instead.
func (s *Server) cookieSecure(r *http.Request) bool {
	if planeOf(r) == planeAdmin {
		if s.cfg.AdminURL != "" {
			return strings.HasPrefix(s.cfg.AdminURL, "https://")
		}
		return r.TLS != nil
	}
	return s.cfg.CookieSecure
}

// sameOrigin reports whether a browser Origin header belongs to this
// deployment for the plane the request arrived on.
func (s *Server) sameOrigin(r *http.Request, origin string) bool {
	if planeOf(r) == planeAdmin {
		if s.cfg.AdminURL != "" {
			return strings.EqualFold(origin, s.cfg.AdminURL)
		}
		// No ADMIN_URL: accept the host the browser actually used. The
		// CSRF token is the real defence; this is belt and braces.
		o := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
		return strings.EqualFold(o, r.Host)
	}
	return strings.EqualFold(origin, s.cfg.BaseURL)
}

// oidcRedirectURL: SSO must come back to whichever plane started it.
func (s *Server) oidcRedirectURL(r *http.Request) string {
	if planeOf(r) == planeAdmin && s.cfg.AdminURL != "" {
		return s.cfg.AdminURL + "/auth/oidc/callback"
	}
	return s.cfg.BaseURL + "/auth/oidc/callback"
}

// needsIdentity keeps session/API-key lookups off the redirect hot path and
// static assets; only these routes ever read the caller's identity.
func needsIdentity(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/auth/") || path == "/mcp"
}
