package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"shortr/internal/ulid"
)

type middleware func(http.Handler) http.Handler

func chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// --- recover -------------------------------------------------------------

// recoverMiddleware ensures a panic in any handler becomes a 500, never a
// crashed process.
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic recovered", "request_id", requestIDFromContext(r.Context()), "panic", rec, "path", r.URL.Path)
				respondError(w, r, ErrInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- request id ------------------------------------------------------------

func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = ulid.New()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- real IP ---------------------------------------------

func (s *Server) realIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := resolveClientIP(r, s.cfg.TrustedProxies, s.cfg.RealIPHeader)
		if s.cfg.RealIPHeader != "" && r.Header.Get(s.cfg.RealIPHeader) != "" && len(s.cfg.TrustedProxies) == 0 {
			s.warnProxy.Do(func() {
				s.log.Warn("request carries a forwarded-IP header but SHORTR_TRUSTED_PROXIES is empty; "+
					"all clients are seen as the proxy's address, so rate limits and analytics are shared",
					"header", s.cfg.RealIPHeader, "remote", r.RemoteAddr)
			})
		}
		ctx := context.WithValue(r.Context(), ctxKeyClientIP, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func resolveClientIP(r *http.Request, trusted []*net.IPNet, header string) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote := net.ParseIP(host)
	if remote == nil || !ipInAny(remote, trusted) {
		return remote
	}

	hv := r.Header.Get(header)
	if hv == "" {
		return remote
	}
	parts := strings.Split(hv, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(parts[i]))
		if candidate == nil {
			continue
		}
		if !ipInAny(candidate, trusted) {
			return candidate
		}
	}
	// all entries trusted (or unparsable): fall back to the leftmost
	first := net.ParseIP(strings.TrimSpace(parts[0]))
	if first != nil {
		return first
	}
	return remote
}

func ipInAny(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func clientScheme(r *http.Request, trusted []*net.IPNet, remoteTrusted bool) string {
	if remoteTrusted {
		if v := r.Header.Get("X-Forwarded-Proto"); v == "http" || v == "https" {
			return v
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// --- logging ---------------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}
func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = 200
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.bytes += int64(n)
	return n, err
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = 200
		}
		dur := time.Since(start)
		level := slog.LevelInfo
		// redirects are extremely high-volume; keep them at debug by default
		//
		isRedirect := r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/api/") &&
			!strings.HasPrefix(r.URL.Path, "/app") && !strings.HasPrefix(r.URL.Path, "/auth") &&
			r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && r.URL.Path != "/metrics"
		if isRedirect {
			level = slog.LevelDebug
		}
		ip := clientIPFromContext(r.Context())
		var ipStr string
		if ip != nil {
			ipStr = ip.String()
		}
		s.metrics.IncHTTPRequest(routeTemplate(r), r.Method, rec.status)
		s.log.Log(r.Context(), level, "request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", dur.Milliseconds(), "bytes", rec.bytes, "ip", ipStr,
		)
	})
}

// routeTemplate collapses path params for metrics cardinality (never logs a
// raw short code as a metric label).
func routeTemplate(r *http.Request) string {
	p := r.URL.Path
	switch {
	case strings.HasPrefix(p, "/api/v1/links/"):
		return "/api/v1/links/:id"
	case strings.HasPrefix(p, "/api/v1/"):
		return p
	case strings.HasPrefix(p, "/auth/"):
		return p
	case strings.HasPrefix(p, "/app"):
		return "/app/*"
	default:
		return "/:code"
	}
}

// --- security headers ------------------------------------------------------

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/app") || r.URL.Path == "/" {
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; font-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		}
		if s.cfg.CookieSecure {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// --- rate limiting -----------------------------------------------------

// rateLimitMiddleware applies limiter keyed by keyFn(r); on exhaustion it
// responds 429 with Retry-After and stops the chain. scope is the metrics
// label.
func (s *Server) rateLimitMiddleware(limiter interface{ Allow(string) bool }, scope string, keyFn func(*http.Request) string) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key != "" && !limiter.Allow(key) {
				s.metrics.IncRateLimited(scope)
				w.Header().Set("Retry-After", "2")
				respondError(w, r, ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// apiRateLimitMiddleware applies RATE_LIMIT_API to every /api/v1/* request,
// keyed by the authenticated user/API-key id when known (so one user's
// traffic can't starve another's), falling back to IP otherwise.
// Runs after withIdentityMiddleware so identity is available.
func (s *Server) apiRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/mcp" {
			next.ServeHTTP(w, r)
			return
		}
		key := ipKeyFn(r)
		if k := apiKeyFromContext(r.Context()); k != nil {
			key = "key:" + k.ID
		} else if u := userFromContext(r.Context()); u != nil {
			key = "user:" + u.ID
		}
		if key != "" && !s.rlAPI.Allow(key) {
			s.metrics.IncRateLimited("api")
			w.Header().Set("Retry-After", "2")
			respondError(w, r, ErrRateLimited)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ipKeyFn(r *http.Request) string {
	ip := clientIPFromContext(r.Context())
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	// IPv6: key by /64 to avoid trivial per-address bypass
	v6 := ip.To16()
	if v6 == nil {
		return ip.String()
	}
	masked := make(net.IP, 16)
	copy(masked[:8], v6[:8])
	return masked.String()
}
