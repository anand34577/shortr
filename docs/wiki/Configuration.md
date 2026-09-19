# Configuration

Every setting is an environment variable prefixed `SHORTR_`. Config is
loaded once at startup and validated fail-fast: an invalid value (or a
missing required one, like `SHORTR_DB_DSN` when `SHORTR_DB_DRIVER=postgres`)
exits the process with a clear error instead of starting half-configured.
Run `shortr config check` at any time to validate and print the resolved
(secret-redacted) configuration.

## Core

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_BASE_URL` | `http://localhost:8080` | Public URL Shortr is served at. Must include scheme; used to build short links and the OIDC redirect URI. |
| `SHORTR_LISTEN` | `:8080` | Address the HTTP server binds to. |
| `SHORTR_DATA_DIR` | `./data` | Where SQLite DB, the generated secret key, GeoIP DB, click spool, and backups live. |
| `SHORTR_SECRET_KEY` | *(auto-generated)* | ≥32 chars, used for HMAC-signing cookies. If unset, a random key is generated once and persisted to `DATA_DIR/secret.key` (mode 0600) — set it explicitly for multi-instance/PostgreSQL deployments so all instances share one key. |
| `SHORTR_SHUTDOWN_TIMEOUT` | `15s` | Grace period for draining in-flight requests, the click writer, and background jobs on SIGTERM/SIGINT. |
| `SHORTR_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error`. |
| `SHORTR_LOG_FORMAT` | `json` | `json` \| `text`. |

## Database

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_DB_DRIVER` | `sqlite` | `sqlite` \| `postgres`. |
| `SHORTR_DB_DSN` | `<DATA_DIR>/shortr.db` | Required (no default) when driver is `postgres`, e.g. `postgres://user:pass@host:5432/shortr?sslmode=disable`. |
| `SHORTR_DB_MAX_CONNS` | `8` (sqlite) / `16` (postgres) | Max open connections; SQLite writes are always serialized through a single connection internally regardless of this value. |

## Networking / reverse proxy

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_TRUSTED_PROXIES` | *(none)* | Comma-separated CIDRs (bare IPs are treated as `/32`) allowed to set the real-IP header. **Required** if you're behind any reverse proxy — otherwise `X-Forwarded-For` is ignored and every click/rate-limit bucket is attributed to the proxy's IP. |
| `SHORTR_REAL_IP_HEADER` | `X-Forwarded-For` | Header to trust for the client IP, once the immediate peer is in `TRUSTED_PROXIES`. |
| `SHORTR_COOKIE_SECURE` | `true` if `BASE_URL` is `https`, else `false` | Forces the `Secure` cookie flag. |
| `SHORTR_SESSION_TTL` | `720h` (30 days) | Session cookie lifetime. |
| `SHORTR_CORS_ORIGINS` | *(none)* | Comma-separated allowlist for cross-origin API access (e.g. a separately-hosted browser extension or SPA). Never a wildcard when credentials are involved. |

## Links

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_CODE_LENGTH` | `7` | 4–16. |
| `SHORTR_CODE_ALPHABET` | `base58` | `base58` \| `base62`. |
| `SHORTR_MAX_URL_LENGTH` | `2048` | 64–8192. |
| `SHORTR_DEFAULT_REDIRECT_STATUS` | `302` | One of `301`, `302`, `307`, `308`; overridable per-link. |
| `SHORTR_ALLOW_PRIVATE_TARGETS` | `false` | Allows short links (and title-fetch/MCP tools) to target private/loopback/link-local IPs. **Leave this false** unless you specifically need internal-network short links — enabling it removes SSRF protection for every user, not just admins. |
| `SHORTR_BLOCKED_DOMAINS` | *(none)* | Comma-separated hostnames that can never be a link target. |
| `SHORTR_MAX_LINKS_PER_USER` | `0` (unlimited) | Per-user link quota. |
| `SHORTR_FETCH_TITLES` | `true` | Best-effort page-title fetch on link creation (SSRF-guarded, 3s timeout, 256KB cap, 3 redirects max). |

## Analytics / privacy

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_IP_MODE` | `anonymize` | `full` (store raw IP) \| `anonymize` (zero the last octet/64 bits) \| `hash` (keyed HMAC, no raw IP ever stored) \| `none` (don't store IP at all). |
| `SHORTR_GEOIP_DB` | *(none)* | Path to a MaxMind GeoLite2/GeoIP2 City `.mmdb` file. Geo columns are empty without one. |
| `SHORTR_CLICK_RETENTION_DAYS` | `365` | Raw click rows older than this are purged daily; `0` disables purging. |
| `SHORTR_ROLLUP_RETENTION_DAYS` | `0` (disabled) | Daily rollup retention, independent of raw click retention. |
| `SHORTR_COUNT_BOTS` | `false` | Whether bot-detected hits (chat-app link unfurlers, crawlers) count toward a link's public click counter. They're always recorded and always excluded from analytics "clicks" unless this is true — see the `is_bot` flag. |

## Rate limiting

Format is `<count>/<window>`, e.g. `200/10s`. `0` disables a limiter.

| Variable | Default |
|---|---|
| `SHORTR_RATE_LIMIT_REDIRECT` | `200/10s` |
| `SHORTR_RATE_LIMIT_API` | `120/60s` (also applies to `/mcp`) |
| `SHORTR_RATE_LIMIT_AUTH` | `10/60s` |
| `SHORTR_RATE_LIMIT_ANON_CREATE` | `0` |

## Accounts & auth

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_REGISTRATION` | `closed` | `closed` \| `open` \| `invite`. |
| `SHORTR_ADMIN_EMAIL` / `SHORTR_ADMIN_PASSWORD` | *(none)* | If both are set and the users table is empty at startup, seeds an initial admin non-interactively (useful for automated deploys). |

### OIDC / SSO

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_OIDC_ENABLED` | `false` | |
| `SHORTR_OIDC_ISSUER` | | Required if enabled; must be `https://` unless `OIDC_INSECURE_HTTP=true`. |
| `SHORTR_OIDC_CLIENT_ID` / `SHORTR_OIDC_CLIENT_SECRET` | | Required if enabled. |
| `SHORTR_OIDC_SCOPES` | `openid email profile` | Space-separated. |
| `SHORTR_OIDC_AUTO_CREATE` | `false` | Create a local account on first SSO login. |
| `SHORTR_OIDC_AUTO_LINK_BY_EMAIL` | `false` | Link an SSO identity to an existing local account with a matching email. |
| `SHORTR_OIDC_REQUIRE_EMAIL_VERIFIED` | `true` | Reject unverified-email identities. |
| `SHORTR_OIDC_ADMIN_GROUP` | | Group name (from `OIDC_GROUPS_CLAIM`) that grants admin on login. |
| `SHORTR_OIDC_GROUPS_CLAIM` | `groups` | |
| `SHORTR_OIDC_DISPLAY_NAME` | `Single Sign-On` | Button label in the login UI. |
| `SHORTR_OIDC_LOCAL_LOGIN` | `true` | Whether local email+password login stays available alongside SSO. |
| `SHORTR_OIDC_ALLOWED_DOMAINS` | *(none)* | Restrict SSO logins to these email domains. |
| `SHORTR_OIDC_INSECURE_HTTP` | `false` | Allows a non-HTTPS issuer — local testing only. |
| `SHORTR_OIDC_REQUIRED` | `false` | If discovery fails at startup, exit instead of retrying lazily. |

## Notifications

### SMTP

| Variable | Default |
|---|---|
| `SHORTR_SMTP_ENABLED` | `false` |
| `SHORTR_SMTP_HOST` / `SHORTR_SMTP_PORT` | *(none)* / `587` |
| `SHORTR_SMTP_USER` / `SHORTR_SMTP_PASS` | *(none)* |
| `SHORTR_SMTP_FROM` | *(none, required if enabled)* |
| `SHORTR_SMTP_USE_TLS` | `true` (STARTTLS) |
| `SHORTR_SMTP_INSECURE` | `false` — skip TLS certificate verification, testing only |

### Gotify

| Variable | Default |
|---|---|
| `SHORTR_GOTIFY_ENABLED` | `false` |
| `SHORTR_GOTIFY_URL` / `SHORTR_GOTIFY_TOKEN` | *(none, both required if enabled)* |

## Observability & maintenance

| Variable | Default | Notes |
|---|---|---|
| `SHORTR_METRICS_ENABLED` | `true` | Prometheus text exposition at `/metrics`. |
| `SHORTR_METRICS_TOKEN` | *(none)* | If set, `/metrics` requires `Authorization: Bearer <token>`. |
| `SHORTR_BACKUP_INTERVAL` | `24h` | Automatic SQLite backup interval; `0` disables. **SQLite only** — set on a PostgreSQL deployment, it logs a warning and never runs; use `pg_dump`/managed snapshots instead. |
| `SHORTR_BACKUP_KEEP` | `7` | How many rotated backups to retain. |

See also: [Backup & Restore](Backup-and-Restore.md), [Security](Security.md).
