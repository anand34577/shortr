// Package config loads and validates all runtime configuration from
// environment variables. Fails fast (returns an error) on
// anything invalid so the process never starts half-configured.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL   string
	BaseHost  string // hostname only, for self-loop checks
	Listen    string
	DataDir   string
	SecretKey []byte

	DBDriver   string // sqlite | postgres
	DBDSN      string
	DBMaxConns int

	TrustedProxies []*net.IPNet
	RealIPHeader   string
	CookieSecure   bool
	SessionTTL     time.Duration

	CodeLength          int
	CodeAlphabet        string
	MaxURLLength        int
	DefaultRedirectCode int
	AllowPrivateTargets bool
	BlockedDomains      []string

	IPMode              string // full | anonymize | hash | none
	GeoIPDB             string
	ClickRetentionDays  int
	RollupRetentionDays int
	CountBots           bool

	RateLimitRedirect   string
	RateLimitAPI        string
	RateLimitAuth       string
	RateLimitAnonCreate string

	Registration    string // closed | open | invite
	MaxLinksPerUser int
	FetchTitles     bool

	OIDCEnabled              bool
	OIDCIssuer               string
	OIDCClientID             string
	OIDCClientSecret         string
	OIDCScopes               []string
	OIDCAutoCreate           bool
	OIDCAutoLinkByEmail      bool
	OIDCRequireEmailVerified bool
	OIDCAdminGroup           string
	OIDCGroupsClaim          string
	OIDCDisplayName          string
	OIDCLocalLogin           bool
	OIDCAllowedDomains       []string
	OIDCInsecureHTTP         bool
	OIDCRequired             bool

	SMTPEnabled  bool
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	SMTPUseTLS   bool
	SMTPInsecure bool

	GotifyEnabled bool
	GotifyURL     string
	GotifyToken   string

	LogLevel  string
	LogFormat string

	MetricsEnabled bool
	MetricsToken   string

	BackupInterval time.Duration
	BackupKeep     int

	ShutdownTimeout time.Duration
	AdminEmail      string
	AdminPassword   string

	CORSOrigins []string
	UIEnabled   bool
}

func env(key, def string) string {
	if v, ok := os.LookupEnv("SHORTR_" + key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv("SHORTR_" + key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("SHORTR_%s: invalid bool %q", key, v)
	}
	return b, nil
}

func envInt(key string, def int) (int, error) {
	v, ok := os.LookupEnv("SHORTR_" + key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("SHORTR_%s: invalid integer %q", key, v)
	}
	return n, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv("SHORTR_" + key)
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("SHORTR_%s: invalid duration %q", key, v)
	}
	return d, nil
}

func envList(key string) []string {
	v := env(key, "")
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Load reads config from the environment, applies defaults, validates, and
// (for SecretKey) persists a generated value to DATA_DIR/secret.key on first run.
func Load() (*Config, error) {
	c := &Config{}
	c.BaseURL = strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/")
	bu, err := url.Parse(c.BaseURL)
	if err != nil || bu.Scheme == "" || bu.Host == "" {
		return nil, fmt.Errorf("SHORTR_BASE_URL: invalid URL %q", c.BaseURL)
	}
	if bu.Scheme != "http" && bu.Scheme != "https" {
		return nil, errors.New("SHORTR_BASE_URL must be http or https")
	}
	c.BaseHost = bu.Hostname()

	c.Listen = env("LISTEN", ":8080")
	c.DataDir = env("DATA_DIR", "./data")
	if err := os.MkdirAll(c.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("cannot create SHORTR_DATA_DIR %q: %w", c.DataDir, err)
	}

	c.DBDriver = env("DB_DRIVER", "sqlite")
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return nil, fmt.Errorf("SHORTR_DB_DRIVER must be sqlite or postgres, got %q", c.DBDriver)
	}
	defaultDSN := filepath.Join(c.DataDir, "shortr.db")
	c.DBDSN = env("DB_DSN", defaultDSN)
	if c.DBDriver == "postgres" && env("DB_DSN", "") == "" {
		return nil, errors.New("SHORTR_DB_DSN is required when SHORTR_DB_DRIVER=postgres")
	}
	defMaxConns := 8
	if c.DBDriver == "postgres" {
		defMaxConns = 16
	}
	if c.DBMaxConns, err = envInt("DB_MAX_CONNS", defMaxConns); err != nil {
		return nil, err
	}

	if err := c.loadSecretKey(); err != nil {
		return nil, err
	}

	tp := envList("TRUSTED_PROXIES")
	for _, cidr := range tp {
		if !strings.Contains(cidr, "/") {
			cidr += "/32"
		}
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("SHORTR_TRUSTED_PROXIES: invalid CIDR %q", cidr)
		}
		c.TrustedProxies = append(c.TrustedProxies, ipnet)
	}
	c.RealIPHeader = env("REAL_IP_HEADER", "X-Forwarded-For")

	cookieSecureDefault := bu.Scheme == "https"
	if c.CookieSecure, err = envBool("COOKIE_SECURE", cookieSecureDefault); err != nil {
		return nil, err
	}
	if c.SessionTTL, err = envDuration("SESSION_TTL", 720*time.Hour); err != nil {
		return nil, err
	}

	if c.CodeLength, err = envInt("CODE_LENGTH", 7); err != nil {
		return nil, err
	}
	if c.CodeLength < 4 || c.CodeLength > 16 {
		return nil, errors.New("SHORTR_CODE_LENGTH must be between 4 and 16")
	}
	c.CodeAlphabet = env("CODE_ALPHABET", "base58")
	if c.CodeAlphabet != "base58" && c.CodeAlphabet != "base62" {
		return nil, errors.New("SHORTR_CODE_ALPHABET must be base58 or base62")
	}
	if c.MaxURLLength, err = envInt("MAX_URL_LENGTH", 2048); err != nil {
		return nil, err
	}
	if c.MaxURLLength < 64 || c.MaxURLLength > 8192 {
		return nil, errors.New("SHORTR_MAX_URL_LENGTH must be between 64 and 8192")
	}
	if c.DefaultRedirectCode, err = envInt("DEFAULT_REDIRECT_STATUS", 302); err != nil {
		return nil, err
	}
	if !validRedirectStatus(c.DefaultRedirectCode) {
		return nil, errors.New("SHORTR_DEFAULT_REDIRECT_STATUS must be one of 301,302,307,308")
	}
	if c.AllowPrivateTargets, err = envBool("ALLOW_PRIVATE_TARGETS", false); err != nil {
		return nil, err
	}
	c.BlockedDomains = envList("BLOCKED_DOMAINS")

	c.IPMode = env("IP_MODE", "anonymize")
	switch c.IPMode {
	case "full", "anonymize", "hash", "none":
	default:
		return nil, errors.New("SHORTR_IP_MODE must be one of full,anonymize,hash,none")
	}
	c.GeoIPDB = env("GEOIP_DB", "")
	if c.ClickRetentionDays, err = envInt("CLICK_RETENTION_DAYS", 365); err != nil {
		return nil, err
	}
	if c.RollupRetentionDays, err = envInt("ROLLUP_RETENTION_DAYS", 0); err != nil {
		return nil, err
	}
	if c.CountBots, err = envBool("COUNT_BOTS", false); err != nil {
		return nil, err
	}

	c.RateLimitRedirect = env("RATE_LIMIT_REDIRECT", "200/10s")
	c.RateLimitAPI = env("RATE_LIMIT_API", "120/60s")
	c.RateLimitAuth = env("RATE_LIMIT_AUTH", "10/60s")
	c.RateLimitAnonCreate = env("RATE_LIMIT_ANON_CREATE", "0")

	c.Registration = env("REGISTRATION", "closed")
	switch c.Registration {
	case "closed", "open", "invite":
	default:
		return nil, errors.New("SHORTR_REGISTRATION must be one of closed,open,invite")
	}
	if c.MaxLinksPerUser, err = envInt("MAX_LINKS_PER_USER", 0); err != nil {
		return nil, err
	}
	if c.FetchTitles, err = envBool("FETCH_TITLES", true); err != nil {
		return nil, err
	}

	if c.OIDCEnabled, err = envBool("OIDC_ENABLED", false); err != nil {
		return nil, err
	}
	c.OIDCIssuer = env("OIDC_ISSUER", "")
	c.OIDCClientID = env("OIDC_CLIENT_ID", "")
	c.OIDCClientSecret = env("OIDC_CLIENT_SECRET", "")
	scopes := env("OIDC_SCOPES", "openid email profile")
	c.OIDCScopes = strings.Fields(scopes)
	if c.OIDCAutoCreate, err = envBool("OIDC_AUTO_CREATE", false); err != nil {
		return nil, err
	}
	if c.OIDCAutoLinkByEmail, err = envBool("OIDC_AUTO_LINK_BY_EMAIL", false); err != nil {
		return nil, err
	}
	if c.OIDCRequireEmailVerified, err = envBool("OIDC_REQUIRE_EMAIL_VERIFIED", true); err != nil {
		return nil, err
	}
	c.OIDCAdminGroup = env("OIDC_ADMIN_GROUP", "")
	c.OIDCGroupsClaim = env("OIDC_GROUPS_CLAIM", "groups")
	c.OIDCDisplayName = env("OIDC_DISPLAY_NAME", "Single Sign-On")
	if c.OIDCLocalLogin, err = envBool("OIDC_LOCAL_LOGIN", true); err != nil {
		return nil, err
	}
	c.OIDCAllowedDomains = envList("OIDC_ALLOWED_DOMAINS")
	if c.OIDCInsecureHTTP, err = envBool("OIDC_INSECURE_HTTP", false); err != nil {
		return nil, err
	}
	if c.OIDCRequired, err = envBool("OIDC_REQUIRED", false); err != nil {
		return nil, err
	}
	if c.OIDCEnabled {
		if c.OIDCIssuer == "" || c.OIDCClientID == "" {
			return nil, errors.New("SHORTR_OIDC_ISSUER and SHORTR_OIDC_CLIENT_ID are required when SHORTR_OIDC_ENABLED=true")
		}
		if !c.OIDCInsecureHTTP && !strings.HasPrefix(c.OIDCIssuer, "https://") {
			return nil, errors.New("SHORTR_OIDC_ISSUER must be https (set SHORTR_OIDC_INSECURE_HTTP=true to override for local testing)")
		}
	}

	if c.SMTPEnabled, err = envBool("SMTP_ENABLED", false); err != nil {
		return nil, err
	}
	c.SMTPHost = env("SMTP_HOST", "")
	if c.SMTPPort, err = envInt("SMTP_PORT", 587); err != nil {
		return nil, err
	}
	c.SMTPUser = env("SMTP_USER", "")
	c.SMTPPass = env("SMTP_PASS", "")
	c.SMTPFrom = env("SMTP_FROM", "")
	if c.SMTPUseTLS, err = envBool("SMTP_USE_TLS", true); err != nil {
		return nil, err
	}
	if c.SMTPInsecure, err = envBool("SMTP_INSECURE", false); err != nil {
		return nil, err
	}
	if c.SMTPEnabled && (c.SMTPHost == "" || c.SMTPFrom == "") {
		return nil, errors.New("SHORTR_SMTP_HOST and SHORTR_SMTP_FROM are required when SHORTR_SMTP_ENABLED=true")
	}

	if c.GotifyEnabled, err = envBool("GOTIFY_ENABLED", false); err != nil {
		return nil, err
	}
	c.GotifyURL = strings.TrimRight(env("GOTIFY_URL", ""), "/")
	c.GotifyToken = env("GOTIFY_TOKEN", "")
	if c.GotifyEnabled && (c.GotifyURL == "" || c.GotifyToken == "") {
		return nil, errors.New("SHORTR_GOTIFY_URL and SHORTR_GOTIFY_TOKEN are required when SHORTR_GOTIFY_ENABLED=true")
	}

	c.LogLevel = env("LOG_LEVEL", "info")
	c.LogFormat = env("LOG_FORMAT", "json")

	if c.MetricsEnabled, err = envBool("METRICS_ENABLED", true); err != nil {
		return nil, err
	}
	c.MetricsToken = env("METRICS_TOKEN", "")

	if c.BackupInterval, err = envDuration("BACKUP_INTERVAL", 24*time.Hour); err != nil {
		return nil, err
	}
	if c.BackupKeep, err = envInt("BACKUP_KEEP", 7); err != nil {
		return nil, err
	}
	if c.ShutdownTimeout, err = envDuration("SHUTDOWN_TIMEOUT", 15*time.Second); err != nil {
		return nil, err
	}

	c.AdminEmail = env("ADMIN_EMAIL", "")
	c.AdminPassword = env("ADMIN_PASSWORD", "")
	c.CORSOrigins = envList("CORS_ORIGINS")
	// UI_ENABLED gates the embedded React admin console (/app and the "/"
	// landing redirect) so a deployment can expose only the redirect hot
	// path and the JSON API — e.g. behind a public reverse proxy that's
	// meant for API/Android-app use only, with the dashboard reserved for
	// an SSH tunnel or a private network. Off by default is unnecessary
	// (the console is fully authenticated), but on request this makes
	// "API/redirects only, in production, publicly" a one-line env flag
	// instead of relying solely on edge routing to hide it.
	if c.UIEnabled, err = envBool("UI_ENABLED", true); err != nil {
		return nil, err
	}

	return c, nil
}

func validRedirectStatus(n int) bool {
	return n == 301 || n == 302 || n == 307 || n == 308
}

func (c *Config) loadSecretKey() error {
	if v := env("SECRET_KEY", ""); v != "" {
		if len(v) < 32 {
			return errors.New("SHORTR_SECRET_KEY must be at least 32 characters")
		}
		c.SecretKey = []byte(v)
		return nil
	}
	path := filepath.Join(c.DataDir, "secret.key")
	if b, err := os.ReadFile(path); err == nil {
		s := strings.TrimSpace(string(b))
		key, err := hex.DecodeString(s)
		if err == nil && len(key) >= 32 {
			c.SecretKey = key
			return nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generating secret key: %w", err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		return fmt.Errorf("writing secret.key: %w", err)
	}
	c.SecretKey = key
	return nil
}

// Redacted returns a map safe to log (secrets masked).
func (c *Config) Redacted() map[string]any {
	mask := func(s string) string {
		if s == "" {
			return ""
		}
		return "***"
	}
	return map[string]any{
		"base_url": c.BaseURL, "listen": c.Listen, "data_dir": c.DataDir,
		"db_driver": c.DBDriver, "db_dsn": maskDSN(c.DBDSN, c.DBDriver),
		"ip_mode": c.IPMode, "registration": c.Registration,
		"oidc_enabled": c.OIDCEnabled, "oidc_issuer": c.OIDCIssuer,
		"oidc_client_secret": mask(c.OIDCClientSecret),
		"smtp_enabled":       c.SMTPEnabled, "smtp_host": c.SMTPHost,
		"gotify_enabled": c.GotifyEnabled, "gotify_url": c.GotifyURL,
		"metrics_enabled": c.MetricsEnabled,
	}
}

func maskDSN(dsn, driver string) string {
	if driver == "sqlite" {
		return dsn
	}
	if i := strings.Index(dsn, "@"); i > 0 {
		return "***@" + dsn[i+1:]
	}
	return "***"
}
