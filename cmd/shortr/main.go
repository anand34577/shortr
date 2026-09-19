// Command shortr is a self-hosted URL shortener: single static binary,
// embedded SQLite by default (or PostgreSQL), embedded React frontend,
// OIDC/local auth, analytics, and optional SMTP/Gotify notifications.
// See PLAN.md for the full design.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	webdist "shortr"
	"shortr/internal/auth"
	"shortr/internal/click"
	"shortr/internal/config"
	"shortr/internal/jobs"
	"shortr/internal/link"
	"shortr/internal/metrics"
	"shortr/internal/notify"
	"shortr/internal/server"
	"shortr/internal/store"
)

// version/commit are set via -ldflags at build time (Makefile / goreleaser).
var (
	version = "dev"
	commit  = "none"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			cmdMigrate(os.Args[2:])
			return
		case "admin":
			cmdAdmin(os.Args[2:])
			return
		case "backup":
			cmdBackup(os.Args[2:])
			return
		case "config":
			cmdConfig(os.Args[2:])
			return
		case "healthcheck":
			cmdHealthcheck(os.Args[2:])
			return
		case "version":
			fmt.Printf("shortr %s (%s)\n", version, commit)
			return
		case "serve", "--serve":
			// fallthrough to normal serve below
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}
	cmdServe()
}

func printUsage() {
	fmt.Println(`shortr - self-hosted URL shortener

Usage:
  shortr [serve]                 Start the server (default)
  shortr migrate                 Apply pending database migrations and exit
  shortr admin create            Create an admin user
  shortr admin reset-password    Reset a user's password
  shortr admin promote           Promote a user to admin
  shortr backup                  Write an on-demand SQLite backup
  shortr config check            Validate configuration and exit
  shortr healthcheck             Exit 0 if the local server is healthy (used by Docker HEALTHCHECK)
  shortr version                 Print version info

Configuration is via SHORTR_* environment variables — see PLAN.md §6.`)
}

func setupLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	log := slog.New(handler)
	slog.SetDefault(log)
	return log
}

func cmdServe() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	log := setupLogger(cfg)
	log.Info("starting shortr", "version", version, "commit", commit)
	log.Info("configuration", "config", cfg.Redacted())

	st, err := store.Open(cfg.DBDriver, cfg.DBDSN, cfg.DataDir, cfg.DBMaxConns)
	if err != nil {
		log.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		seedAdmin(st, cfg, log)
	}

	m := metrics.New(version, commit)

	geo, err := click.OpenGeoDB(cfg.GeoIPDB)
	if err != nil {
		log.Warn("GeoIP database could not be opened; geo features disabled", "path", cfg.GeoIPDB, "error", err)
	}

	clickWriter := click.NewWriter(st, geo, click.WriterConfig{
		IPMode: cfg.IPMode, Secret: cfg.SecretKey, CountBots: cfg.CountBots,
		SpoolDir: filepath.Join(cfg.DataDir, "clicks-spool"),
	}, m, log)
	clickCtx, clickCancel := context.WithCancel(context.Background())
	go clickWriter.Run(clickCtx)

	linkSvc := link.NewService(st, link.Config{
		BaseURL: cfg.BaseURL, BaseHost: cfg.BaseHost, CodeLength: cfg.CodeLength, Alphabet: cfg.CodeAlphabet,
		MaxURLLength: cfg.MaxURLLength, AllowPrivateTargets: cfg.AllowPrivateTargets, BlockedDomains: cfg.BlockedDomains,
		DefaultRedirectStatus: cfg.DefaultRedirectCode, MaxLinksPerUser: cfg.MaxLinksPerUser,
	})

	var smtpSender *notify.SMTPSender
	if cfg.SMTPEnabled {
		smtpSender = notify.NewSMTPSender(notify.SMTPConfig{
			Enabled: true, Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass,
			From: cfg.SMTPFrom, UseTLS: cfg.SMTPUseTLS, Insecure: cfg.SMTPInsecure,
		})
	} else {
		smtpSender = notify.NewSMTPSender(notify.SMTPConfig{})
	}
	var gotifySender *notify.GotifySender
	if cfg.GotifyEnabled {
		gotifySender = notify.NewGotifySender(notify.GotifyConfig{Enabled: true, URL: cfg.GotifyURL, Token: cfg.GotifyToken})
	} else {
		gotifySender = notify.NewGotifySender(notify.GotifyConfig{})
	}
	notifier := notify.New(st, smtpSender, gotifySender, log)

	var oidcMgr *auth.Manager
	if cfg.OIDCEnabled {
		oidcMgr = auth.NewManager(auth.OIDCConfig{
			Issuer: cfg.OIDCIssuer, ClientID: cfg.OIDCClientID, ClientSecret: cfg.OIDCClientSecret,
			Scopes: cfg.OIDCScopes, RedirectURL: strings.TrimRight(cfg.BaseURL, "/") + "/auth/oidc/callback",
			InsecureHTTP: cfg.OIDCInsecureHTTP, RequireEmailVerified: cfg.OIDCRequireEmailVerified,
			AllowedDomains: cfg.OIDCAllowedDomains,
		})
		if !oidcMgr.Ready(context.Background()) {
			log.Warn("OIDC provider discovery failed at startup; will retry lazily", "issuer", cfg.OIDCIssuer)
			if cfg.OIDCRequired {
				log.Error("SHORTR_OIDC_REQUIRED=true and discovery failed; exiting")
				os.Exit(1)
			}
		}
	}

	spaFS := spaSubFS(log)

	srv := server.New(server.Deps{
		Config: cfg, Store: st, Links: linkSvc, ClickWriter: clickWriter, Notifier: notifier,
		OIDC: oidcMgr, Metrics: m, Log: log, SPAFiles: spaFS, Version: version,
	})

	jobsCtx, jobsCancel := context.WithCancel(context.Background())
	jobRunner := jobs.New(jobs.Config{
		Store: st, Notifier: notifier, Log: log,
		ClickRetentionDays: cfg.ClickRetentionDays, RollupRetentionDays: cfg.RollupRetentionDays,
		BackupInterval: cfg.BackupInterval, BackupKeep: cfg.BackupKeep, DataDir: cfg.DataDir, DBDriver: cfg.DBDriver,
	})
	go jobRunner.Run(jobsCtx)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Error("server error", "error", err)
	case sig := <-sigCh:
		log.Info("received signal, shutting down", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("graceful shutdown timed out", "error", err)
	}
	clickWriter.Close()
	<-time.After(300 * time.Millisecond) // let the writer's final flush land
	clickCancel()
	jobsCancel()
	log.Info("shutdown complete")
}

// spaSubFS returns the embedded frontend build (see webdist.go at the
// module root — go:embed can't reach web/dist with ".." from here).
func spaSubFS(log *slog.Logger) fs.FS {
	sub, err := webdist.FS()
	if err != nil {
		log.Warn("embedded frontend not available", "error", err)
		return nil
	}
	return sub
}

func seedAdmin(st *store.Store, cfg *config.Config, log *slog.Logger) {
	ctx := context.Background()
	n, err := st.CountUsers(ctx)
	if err != nil || n > 0 {
		return
	}
	email, err := normalizeEmail(cfg.AdminEmail)
	if err != nil {
		log.Warn("SHORTR_ADMIN_EMAIL is invalid, skipping seed admin", "error", err)
		return
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Warn("failed to hash SHORTR_ADMIN_PASSWORD", "error", err)
		return
	}
	now := time.Now()
	u := &store.User{Email: email, Name: "Admin", PasswordHash: &hash, Role: "admin", Status: "active", EmailVerified: true, RoleLocked: true, PasswordChangedAt: &now}
	if err := st.CreateUser(ctx, u); err != nil {
		log.Warn("failed to seed admin user", "error", err)
		return
	}
	log.Info("seeded initial admin user from SHORTR_ADMIN_EMAIL", "email", email)
}

func cmdHealthcheck(_ []string) {
	listen := os.Getenv("SHORTR_LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	addr := listen
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	_, port, err := net.SplitHostPort(addr)
	if err == nil {
		addr = "127.0.0.1:" + port
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		os.Exit(1)
	}
	os.Exit(0)
}

func cmdConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	fs.Parse(args) //nolint:errcheck
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	fmt.Println("configuration OK")
	for k, v := range cfg.Redacted() {
		fmt.Printf("  %-24s %v\n", k, v)
	}
}

func normalizeEmail(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || !strings.Contains(s, "@") {
		return "", fmt.Errorf("invalid email %q", s)
	}
	return s, nil
}
