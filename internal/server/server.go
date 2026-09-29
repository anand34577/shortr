package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"shortr/internal/auth"
	"shortr/internal/click"
	"shortr/internal/config"
	"shortr/internal/link"
	"shortr/internal/metrics"
	"shortr/internal/notify"
	"shortr/internal/ratelimit"
	"shortr/internal/store"
)

type Server struct {
	cfg         *config.Config
	store       *store.Store
	links       *link.Service
	clickWriter *click.Writer
	notifier    *notify.Notifier
	oidc        *auth.Manager // nil when OIDC disabled
	metrics     *metrics.Registry

	rlRedirect *ratelimit.Limiter
	rlAPI      *ratelimit.Limiter
	rlAuth     *ratelimit.Limiter
	rlPassword *ratelimit.Limiter
	setupMu    sync.Mutex
	warnProxy  sync.Once

	log       *slog.Logger
	handler      http.Handler // public plane (or everything in single-port mode)
	adminHandler http.Handler // nil unless SHORTR_ADMIN_LISTEN is set
	http         *http.Server
	adminHTTP    *http.Server
	sudoMu    sync.Mutex
	sudoUntil map[string]time.Time // session ID -> end of re-auth window
	liveSet   atomic.Pointer[liveSettings]
	spaFS     fs.FS                // nil = SPA disabled (e.g. some tests)
	startTime time.Time
	version   string
	commit    string
}

type Deps struct {
	Config      *config.Config
	Store       *store.Store
	Links       *link.Service
	ClickWriter *click.Writer
	Notifier    *notify.Notifier
	OIDC        *auth.Manager
	Metrics     *metrics.Registry
	Log         *slog.Logger
	SPAFiles    fs.FS
	Version     string
	Commit      string
}

func New(d Deps) *Server {
	rr, _ := ratelimit.ParseRate(d.Config.RateLimitRedirect)
	ra, _ := ratelimit.ParseRate(d.Config.RateLimitAPI)
	rauth, _ := ratelimit.ParseRate(d.Config.RateLimitAuth)

	s := &Server{
		cfg: d.Config, store: d.Store, links: d.Links, clickWriter: d.ClickWriter,
		notifier: d.Notifier, oidc: d.OIDC, metrics: d.Metrics,
		rlRedirect: ratelimit.New(rr), rlAPI: ratelimit.New(ra), rlAuth: ratelimit.New(rauth),
		rlPassword: ratelimit.New(ratelimit.Rate{N: 5, Interval: time.Minute}),
		log:        d.Log, spaFS: d.SPAFiles, startTime: time.Now(), version: d.Version, commit: d.Commit,
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	s.reloadSettings(context.Background())
	s.routes()
	go s.gcRateLimiters()
	s.http = newHTTPServer(d.Config.Listen, s.handler)
	if s.adminHandler != nil {
		s.adminHTTP = newHTTPServer(d.Config.AdminListen, s.adminHandler)
	}
	return s
}

func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 * 1024,
	}
}

// ListenAndServe runs the public listener and, in split mode, the admin
// listener; it returns when either fails.
func (s *Server) ListenAndServe() error {
	if s.adminHTTP == nil {
		s.log.Info("listening", "addr", s.cfg.Listen, "base_url", s.cfg.BaseURL, "ui", s.cfg.UIEnabled)
		return s.http.ListenAndServe()
	}
	s.log.Info("listening (public: redirects + token API)", "addr", s.cfg.Listen, "base_url", s.cfg.BaseURL,
		"public_api", s.cfg.PublicAPI, "public_admin_api", s.cfg.PublicAdminAPI)
	s.log.Info("listening (admin: UI + full API)", "addr", s.cfg.AdminListen, "admin_url", s.cfg.AdminURL)
	errc := make(chan error, 2)
	go func() { errc <- s.http.ListenAndServe() }()
	go func() { errc <- s.adminHTTP.ListenAndServe() }()
	return <-errc
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.adminHTTP != nil {
		_ = s.adminHTTP.Shutdown(ctx)
	}
	return s.http.Shutdown(ctx)
}

// gcRateLimiters periodically evicts idle buckets from the in-memory rate
// limiters so their maps don't grow unbounded under a scanning attack
//. Runs for the process lifetime; no cancellation needed
// since it holds no resources that need closing on shutdown.
func (s *Server) gcRateLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.rlRedirect.GC(10 * time.Minute)
		s.rlAPI.GC(10 * time.Minute)
		s.rlAuth.GC(10 * time.Minute)
		s.rlPassword.GC(10 * time.Minute)
		s.links.PruneHits(time.Hour)
	}
}

// --- request id ---------------------------------------------------------

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyClientIP
	ctxKeyUser
	ctxKeySession
	ctxKeyAPIKey
)

func requestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

func clientIPFromContext(ctx context.Context) net.IP {
	v, _ := ctx.Value(ctxKeyClientIP).(net.IP)
	return v
}

func userFromContext(ctx context.Context) *store.User {
	v, _ := ctx.Value(ctxKeyUser).(*store.User)
	return v
}

func sessionFromContext(ctx context.Context) *store.Session {
	v, _ := ctx.Value(ctxKeySession).(*store.Session)
	return v
}
