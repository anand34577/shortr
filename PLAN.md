# Shortr — URL Shortener: Plan, Design & Architecture

> Status: **Design (v1.0)** · Date: 2026-09-18 · Owner: Anand
> This is the single source of truth for what gets built, how, and why. Nothing is implemented yet.

---

## Table of Contents

1. [Goals & Non-Goals](#1-goals--non-goals)
2. [Guiding Principles](#2-guiding-principles)
3. [Technology Choices (with justification)](#3-technology-choices)
4. [High-Level Architecture](#4-high-level-architecture)
5. [Repository Structure](#5-repository-structure)
6. [Configuration](#6-configuration)
7. [Data Model](#7-data-model)
8. [Short Code Design](#8-short-code-design)
9. [Redirect Hot Path](#9-redirect-hot-path)
10. [Analytics & Click Recording](#10-analytics--click-recording)
11. [Authentication & Authorization](#11-authentication--authorization)
12. [OIDC (OpenID Connect)](#12-oidc-openid-connect)
13. [HTTP API (v1)](#13-http-api-v1)
14. [Frontend (React) Design](#14-frontend-react-design)
15. [UI / UX Specification](#15-ui--ux-specification)
16. [Validation Rules](#16-validation-rules)
17. [Error Handling & Error Codes](#17-error-handling--error-codes)
18. [Security](#18-security)
19. [Traffic Handling, Rate Limiting & Performance](#19-traffic-handling-rate-limiting--performance)
20. [Reliability, Failure Modes & Recovery](#20-reliability-failure-modes--recovery)
21. [Observability](#21-observability)
22. [Deployment](#22-deployment)
23. [Nginx Proxy Manager Integration](#23-nginx-proxy-manager-integration)
24. [Edge Cases Catalogue](#24-edge-cases-catalogue)
25. [Limitations](#25-limitations)
26. [Future Clients: Android App & Browser Extension](#26-future-clients-android-app--browser-extension)
27. [Testing Strategy](#27-testing-strategy)
28. [Implementation Phases](#28-implementation-phases)
29. [Open Decisions (defaults chosen)](#29-open-decisions-defaults-chosen)
30. [Glossary](#30-glossary)

---

## 1. Goals & Non-Goals

### 1.1 Goals

| # | Goal | Measure |
|---|------|---------|
| G1 | Single static binary, zero runtime dependencies | `./shortr` starts on Linux/macOS/Windows (amd64/arm64) with no installed libs; CGO disabled |
| G2 | Docker image | `< 30 MB` image, `distroless/static` base, non-root |
| G3 | Embedded DB by default, PostgreSQL optional | `DB_DRIVER=sqlite` (default) or `DB_DRIVER=postgres`; same feature set |
| G4 | Fast redirects | p99 < 5 ms in-process for cached links; ≥ 10k redirects/s on a 2-vCPU box |
| G5 | Full analytics per click | timestamp, IP (raw or anonymised), country (optional offline GeoIP), device type, OS, browser, referrer, UTM, bot flag |
| G6 | Multi-user | Users own links; admins manage everything; per-user API keys |
| G7 | Auth: local + OIDC | Email/password, OIDC with auto-create toggle, account linking/unlinking |
| G8 | Modern UI | React SPA, dark/light, responsive, keyboard accessible, WCAG 2.2 AA |
| G9 | Reverse-proxy friendly | Correct client IP behind Nginx Proxy Manager / any proxy via trusted-proxy CIDRs |
| G10 | Client-ready API | Stable REST API + API keys for Android app & browser extension |
| G11 | Robustness | Graceful shutdown, panic recovery, no data loss on crash for committed clicks, migrations, backups |
| G12 | No 3rd-party services | No external calls at runtime except user-configured OIDC provider and the user's own target URLs for optional title fetch |

### 1.2 Non-Goals (v1)

- Multi-region / clustered write scaling (single node; horizontally scale read-only replicas is out of scope).
- Custom branded domains per user (single base domain in v1; schema leaves room for it).
- A/B testing, link rotation, geo-targeted redirects (schema leaves room; not built).
- Email sending (password reset by email). v1 uses admin-reset and OIDC; SMTP is a v1.1 item.
- Payment/billing/quotas beyond simple per-user link limit.
- Real-time WebSocket dashboards (polling with TanStack Query is sufficient).

---

## 2. Guiding Principles

1. **Standard library first.** Go's `net/http` (1.22+ pattern routing), `database/sql`, `log/slog`, `embed`, `crypto/*`. Add a dependency only when the stdlib version would be a security risk or more than ~200 lines.
2. **One process, one binary.** Frontend build is embedded with `//go:embed`. No sidecars.
3. **Hot path is sacred.** `GET /{code}` touches: in-memory cache → (miss) one indexed DB read → non-blocking channel send. Nothing else.
4. **Write once, where all callers route through.** One `Store` with dialect awareness, not two repositories.
5. **Fail loudly at startup, fail soft at runtime.** Bad config = refuse to start. Bad analytics write = log + drop that click, never fail the redirect.
6. **Every trust boundary validates.** All user input validated server-side; the frontend validation is UX only.
7. **Mark shortcuts.** Deliberate simplifications are tagged `// ponytail:` in code with the upgrade path.

---

## 3. Technology Choices

### 3.1 Backend — Go 1.23+

| Concern | Choice | Why not the alternative |
|---------|--------|-------------------------|
| HTTP router | `net/http` `ServeMux` with method+pattern routes (`GET /api/v1/links/{id}`) | chi/gin/echo add nothing needed; stdlib routing since 1.22 covers path params & method matching |
| SQLite driver | `modernc.org/sqlite` (pure Go, CGO-free) | `mattn/go-sqlite3` needs CGO → breaks static cross-compile and `distroless/static` |
| PostgreSQL driver | `github.com/jackc/pgx/v5/stdlib` | Fastest, maintained, works through `database/sql` so the Store stays uniform |
| Migrations | Embedded `.sql` files, tiny in-house runner (`schema_migrations` table) | goose/golang-migrate are fine but ~40 lines does the job; two dialect folders |
| Password hashing | `golang.org/x/crypto/argon2` (argon2id) | bcrypt acceptable but argon2id is current OWASP first choice; x/crypto is quasi-stdlib |
| OIDC | `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2` | Hand-rolling JWT/JWKS verification is a security path; not the place to be lazy |
| User-Agent parsing | `github.com/mileusna/useragent` | Zero deps, ~1 file, good enough for device/OS/browser. Upgrade path: uap-go if regex coverage needed |
| GeoIP (optional) | `github.com/oschwald/maxminddb-golang` reading a user-supplied `.mmdb` file | Offline, no network. If no file configured, country = empty. **Not** a 3rd-party service call |
| Metrics | Hand-written Prometheus text exposition over `expvar`-style atomic counters | `client_golang` pulls ~10 packages; we need ~12 series. Upgrade when histograms with labels are required |
| Logging | `log/slog` JSON handler | stdlib |
| QR codes | Client-side (`qrcode.react`) for UI; server-side `github.com/skip2/go-qrcode` for `/api/v1/links/{id}/qr.png` so Android/extension can fetch an image | Small, no deps |
| Rate limiting | In-house token bucket keyed by IP / API key (`sync.Map` + periodic sweep) | `golang.org/x/time/rate` is also fine; in-house is ~60 lines and lets us evict |
| Cache | In-house LRU (`container/list` + map + `sync.Mutex`), sharded 16 ways | ristretto/bigcache are overkill for < 1M entries |
| ID generation | `crypto/rand` for short codes; ULIDs (in-house 40 lines) for row IDs where DB doesn't generate | Sortable, no coordination |
| Validation | Hand-written per field; no reflection validator | Rules are few and specific |
| Testing | `testing` + `net/http/httptest` + in-memory SQLite | No testify needed |

Total direct Go dependencies: **7** (`modernc.org/sqlite`, `pgx`, `x/crypto`, `x/oauth2`, `go-oidc`, `useragent`, `maxminddb-golang`, `go-qrcode`). All pure Go. `CGO_ENABLED=0` builds.

### 3.2 Frontend — React 19 + TypeScript + Vite

| Concern | Choice | Why |
|---------|--------|-----|
| Build | Vite 6 | Fast, standard, produces static `dist/` we embed |
| UI kit | **shadcn/ui** (Radix primitives + Tailwind v4) | Accessible primitives (Dialog, Popover, Combobox, Toast), copy-in components (no runtime lock-in), modern look, dark mode via CSS vars |
| Styling | Tailwind CSS v4 | Design tokens as CSS variables; tiny output |
| Icons | `lucide-react` | Pairs with shadcn |
| Data fetching | TanStack Query v5 | Caching, retries, optimistic updates, polling for analytics |
| Routing | React Router v7 (data mode) | Standard |
| Forms | `react-hook-form` + `zod` | Schema validation mirrored from server rules |
| Charts | `recharts` | Time-series + bar + pie; adequate and tree-shakeable |
| Tables | TanStack Table v8 | Sorting/filtering/column visibility for link list |
| QR | `qrcode.react` | SVG QR in dialog, download as PNG/SVG |
| Dates | `date-fns` | Light |
| Toasts | `sonner` (shadcn default) | |

No Redux/MobX. Server state = TanStack Query; UI state = React state + URL search params.

### 3.3 Why not Next.js / SSR?
Single-binary goal. A pure SPA embedded in Go is one artifact and works from `file://`-like contexts (extension) and on any host. The app is behind login, so SEO is irrelevant. The public pages (password-protected link page, 404, expired page) are rendered by Go `html/template` so they work without JS and are trivially cacheable.

---

## 4. High-Level Architecture

```
                         ┌─────────────────────────────────────────────────────────┐
                         │                    shortr (single binary)               │
  Internet               │                                                         │
    │                    │  ┌──────────────┐    ┌──────────────────────────────┐   │
    │  HTTPS             │  │ Static SPA   │    │  HTTP Server (net/http)      │   │
    ▼                    │  │ embed.FS     │◄───┤  middleware chain:           │   │
┌──────────────┐  HTTP   │  │ /app/*       │    │   recover → reqid → realip → │   │
│ Nginx Proxy  │────────►│  └──────────────┘    │   logging → security headers │   │
│ Manager      │         │                      │   → ratelimit → auth         │   │
│ (TLS, ACME)  │         │  ┌──────────────┐    └───────┬──────────────────────┘   │
└──────────────┘         │  │ Public pages │            │                          │
                         │  │ html/template│◄───────────┼───────┐                  │
                         │  │ 404/410/pw   │            │       │                  │
                         │  └──────────────┘    ┌───────▼──────▼────────┐          │
                         │                      │ Handlers              │          │
                         │  ┌──────────────┐    │  /{code}  (redirect)  │          │
                         │  │ Link Cache   │◄───┤  /api/v1/* (JSON)     │          │
                         │  │ sharded LRU  │    │  /auth/*  (session,   │          │
                         │  └──────┬───────┘    │           oidc)       │          │
                         │         │            └───────┬───────────────┘          │
                         │         │                    │                          │
                         │  ┌──────▼────────────────────▼──────────┐               │
                         │  │ Service layer                        │               │
                         │  │  links · users · auth · oidc ·       │               │
                         │  │  analytics · apikeys · settings      │               │
                         │  └──────┬───────────────────┬───────────┘               │
                         │         │                   │  click events (chan)      │
                         │  ┌──────▼───────┐    ┌──────▼────────────────┐          │
                         │  │ Store        │    │ Click Writer (worker) │          │
                         │  │ database/sql │◄───┤ batch insert every    │          │
                         │  │ sqlite|pg    │    │ 250ms or 500 rows     │          │
                         │  └──────┬───────┘    └───────────────────────┘          │
                         │         │                                               │
                         │  ┌──────▼────────────────────────────────┐              │
                         │  │ Background jobs (time.Ticker)         │              │
                         │  │  rollups · retention · expiry sweep · │              │
                         │  │  session GC · ratelimit GC · backup   │              │
                         │  └───────────────────────────────────────┘              │
                         └───────────────┬─────────────────────────────────────────┘
                                         │
                            ┌────────────▼─────────────┐      ┌──────────────────┐
                            │ SQLite file (WAL)        │  or  │ PostgreSQL 14+   │
                            │ ./data/shortr.db         │      │ (user-provided)  │
                            └──────────────────────────┘      └──────────────────┘
```

### 4.1 Request flows

**Redirect** `GET /abc123`
1. Middleware: recover, request-id, real-ip (trusted proxy), minimal log, rate-limit (per IP, generous), security headers.
2. Handler: normalise code (trim, reject if contains chars outside alphabet) → cache lookup.
3. Cache miss → `Store.GetLinkByCode` (single indexed query) → populate cache (including negative entries for 60 s).
4. Checks in order: exists? → deleted? (410) → disabled? (404 page "link disabled") → expired by time? (410 page) → expired by clicks? (410) → password? (render password page, 200) → OK.
5. `302 Found` (default; configurable per link to `301`/`307`/`308`) with `Location`, `Cache-Control: private, no-store`, `Referrer-Policy: unsafe-url` (so downstream sees the short domain as referrer — configurable).
6. Non-blocking `select { case clickCh <- ev: default: droppedCounter++ }`.
7. Bots (by UA) are still redirected but flagged; whether they count toward `click_count` and click-expiry is a setting (default: no).

**API** `POST /api/v1/links`
1. Same middleware + auth (session cookie **or** `Authorization: Bearer sk_...`) + CSRF check for cookie sessions.
2. Decode JSON with `DisallowUnknownFields`, limit body to 64 KB.
3. Validate → service → store (transaction) → cache invalidate → JSON 201.

**SPA** `GET /app/*` → serve `index.html` (history fallback), assets with immutable cache headers (hashed filenames).

---

## 5. Repository Structure

```
shortr/
├── cmd/shortr/main.go              # wire config → store → services → server; flags: serve (default), migrate, admin reset-password, backup
├── internal/
│   ├── config/config.go            # env + optional YAML/JSON file; validation; Print() with secrets redacted
│   ├── server/
│   │   ├── server.go               # http.Server, routes, graceful shutdown
│   │   ├── middleware.go           # recover, reqid, realip, logging, secheaders, ratelimit, auth, csrf, cors
│   │   ├── redirect.go             # GET /{code}
│   │   ├── api_links.go            # /api/v1/links*
│   │   ├── api_analytics.go        # /api/v1/links/{id}/stats*, /api/v1/stats
│   │   ├── api_users.go            # /api/v1/users*, /api/v1/me
│   │   ├── api_apikeys.go          # /api/v1/apikeys*
│   │   ├── api_settings.go         # /api/v1/settings (admin)
│   │   ├── auth.go                 # /auth/login, /auth/logout, /auth/setup, /auth/oidc/*
│   │   ├── pages.go                # html/template public pages (404, 410, password, disabled)
│   │   ├── spa.go                  # embedded SPA serving
│   │   ├── respond.go              # JSON envelope helpers, error mapping
│   │   └── templates/*.html
│   ├── link/
│   │   ├── service.go              # create/update/delete/list, alias rules, expiry, password
│   │   ├── code.go                 # code generation + alphabet + reserved words
│   │   └── cache.go                # sharded LRU with TTL + negative cache
│   ├── click/
│   │   ├── event.go                # ClickEvent struct
│   │   ├── writer.go               # channel consumer, batching, backpressure
│   │   ├── ua.go                   # UA → device/os/browser/bot
│   │   └── geo.go                  # optional maxminddb lookup
│   ├── auth/
│   │   ├── password.go             # argon2id hash/verify, policy
│   │   ├── session.go              # opaque tokens, cookie, store-backed
│   │   ├── apikey.go               # sk_ prefix, sha256 stored, last_used
│   │   └── oidc.go                 # discovery, PKCE, state/nonce, callback, link/unlink
│   ├── user/service.go             # users, roles, first-run setup
│   ├── analytics/
│   │   ├── query.go                # time-series, breakdowns, top referrers, geo
│   │   └── rollup.go               # daily aggregation job
│   ├── store/
│   │   ├── store.go                # Store struct, Open(), dialect, tx helper, Ping
│   │   ├── sql.go                  # q() dialect-placeholder rewriting ($1 vs ?), upsert helpers
│   │   ├── migrate.go              # embedded migration runner
│   │   ├── migrations/sqlite/0001_init.sql ...
│   │   ├── migrations/postgres/0001_init.sql ...
│   │   ├── links.go · clicks.go · users.go · sessions.go · apikeys.go · oidc.go · settings.go · audit.go
│   ├── ratelimit/bucket.go         # token bucket map with GC
│   ├── metrics/metrics.go          # atomic counters + /metrics text
│   ├── jobs/scheduler.go           # ticker-driven jobs with jitter + single-flight
│   ├── validate/validate.go        # URL, alias, email, password helpers
│   └── ulid/ulid.go                # 40-line ULID
├── web/                            # Vite + React app
│   ├── src/
│   │   ├── app/ (router, providers, layout)
│   │   ├── features/{links,analytics,auth,settings,users,apikeys}/
│   │   ├── components/ui/          # shadcn generated
│   │   ├── lib/api.ts              # typed fetch client, error mapping
│   │   └── lib/schemas.ts          # zod schemas mirroring server validation
│   ├── index.html · vite.config.ts · tailwind config in CSS
│   └── dist/                       # build output (gitignored), embedded by Go
├── deploy/
│   ├── Dockerfile                  # multistage: node build → go build → distroless/static
│   ├── docker-compose.yml          # sqlite default
│   ├── docker-compose.postgres.yml
│   ├── docker-compose.npm.yml      # with Nginx Proxy Manager on same network
│   ├── shortr.service              # systemd unit
│   └── nginx-advanced.conf         # NPM "Advanced" tab snippet
├── docs/ (this file moves here as ARCHITECTURE.md; plus API.md generated OpenAPI)
├── Makefile                        # build, test, lint, release (goreleaser optional)
├── go.mod · go.sum
└── README.md
```

Package rule: `server` imports services; services import `store`; nothing imports `server`. `store` knows SQL; nobody else does.

---

## 6. Configuration

Precedence: **flags > env > config file > defaults**. Config file optional (`--config shortr.yaml`, JSON also accepted since stdlib has no YAML — **decision: JSON or env only; no YAML dependency**). Every value is printed at startup (secrets redacted). Invalid config → exit code 2 with a clear message.

| Env | Default | Description / Validation |
|-----|---------|--------------------------|
| `SHORTR_BASE_URL` | `http://localhost:8080` | Public URL used to build short links. Must be absolute, http(s), no path (v1). Trailing slash stripped. |
| `SHORTR_LISTEN` | `:8080` | Listen address |
| `SHORTR_DATA_DIR` | `./data` | SQLite file, backups, GeoIP db location. Created if missing (0700). |
| `SHORTR_DB_DRIVER` | `sqlite` | `sqlite` \| `postgres` |
| `SHORTR_DB_DSN` | `$DATA_DIR/shortr.db` | For postgres: `postgres://user:pass@host:5432/shortr?sslmode=require`. Required if driver=postgres. |
| `SHORTR_DB_MAX_CONNS` | sqlite: 1 writer / 8 readers; pg: 16 | See §20.3 |
| `SHORTR_SECRET_KEY` | *(generated & persisted to `$DATA_DIR/secret.key` on first run)* | 32+ bytes. Used for cookie signing, CSRF, OIDC state HMAC. If provided must be ≥ 32 chars. |
| `SHORTR_TRUSTED_PROXIES` | *(empty = trust nobody)* | Comma-separated CIDRs, e.g. `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.1/32`. See §23. |
| `SHORTR_REAL_IP_HEADER` | `X-Forwarded-For` | Alternative `X-Real-IP`, `CF-Connecting-IP`. Only honoured from trusted proxies. |
| `SHORTR_COOKIE_SECURE` | auto (true if BASE_URL is https) | Force cookie `Secure` flag |
| `SHORTR_SESSION_TTL` | `720h` (30 d) | Idle-refresh sliding window |
| `SHORTR_CODE_LENGTH` | `7` | 4–16. Auto-generated code length |
| `SHORTR_CODE_ALPHABET` | `base58` | `base62` \| `base58` (no 0/O/l/I — default for human readability) |
| `SHORTR_MAX_URL_LENGTH` | `2048` | 64–8192 |
| `SHORTR_DEFAULT_REDIRECT_STATUS` | `302` | 301/302/307/308 |
| `SHORTR_ALLOW_PRIVATE_TARGETS` | `false` | Allow targets resolving to private/loopback IPs (SSRF/open-redirect control). Hostname check only at create time (no DNS), plus literal IP check. |
| `SHORTR_BLOCKED_DOMAINS` | *(empty)* | Comma list; also editable in admin settings (DB wins) |
| `SHORTR_IP_MODE` | `anonymize` | `full` \| `anonymize` (IPv4 /24, IPv6 /48 zeroed) \| `hash` (HMAC-SHA256 with secret, daily salt) \| `none` |
| `SHORTR_GEOIP_DB` | *(empty)* | Path to `GeoLite2-Country.mmdb` or City. Optional. |
| `SHORTR_CLICK_RETENTION_DAYS` | `365` | 0 = forever. Raw clicks older are deleted after rollup |
| `SHORTR_ROLLUP_RETENTION_DAYS` | `0` | Daily aggregates retention |
| `SHORTR_COUNT_BOTS` | `false` | Bot clicks increment counters / click-expiry |
| `SHORTR_RATE_LIMIT_REDIRECT` | `200/10s` | Per client IP |
| `SHORTR_RATE_LIMIT_API` | `120/60s` | Per user/API key |
| `SHORTR_RATE_LIMIT_AUTH` | `10/60s` | Per IP for login, setup, oidc start |
| `SHORTR_RATE_LIMIT_ANON_CREATE` | `0` | Anonymous link creation (0 = disabled) |
| `SHORTR_REGISTRATION` | `closed` | `closed` \| `open` \| `invite` (local sign-up) |
| `SHORTR_MAX_LINKS_PER_USER` | `0` | 0 = unlimited; admin override per user |
| `SHORTR_FETCH_TITLES` | `true` | Fetch `<title>` of target on create (outbound request, 3 s timeout, 256 KB cap, SSRF-guarded). Off = no outbound traffic ever |
| `SHORTR_OIDC_ENABLED` | `false` | |
| `SHORTR_OIDC_ISSUER` | | Discovery URL base, https required unless `SHORTR_OIDC_INSECURE_HTTP=true` |
| `SHORTR_OIDC_CLIENT_ID` / `_SECRET` | | Secret optional for public clients w/ PKCE |
| `SHORTR_OIDC_SCOPES` | `openid email profile` | |
| `SHORTR_OIDC_AUTO_CREATE` | `false` | Auto-provision a user on first OIDC login |
| `SHORTR_OIDC_AUTO_LINK_BY_EMAIL` | `false` | Link to an existing local user with the same **verified** email automatically |
| `SHORTR_OIDC_REQUIRE_EMAIL_VERIFIED` | `true` | Reject tokens where `email_verified != true` |
| `SHORTR_OIDC_ADMIN_GROUP` | | Claim value granting admin (see `OIDC_GROUPS_CLAIM`) |
| `SHORTR_OIDC_GROUPS_CLAIM` | `groups` | |
| `SHORTR_OIDC_DISPLAY_NAME` | `Single Sign-On` | Button label |
| `SHORTR_OIDC_LOCAL_LOGIN` | `true` | If false, hide local login form (admin can still use `?local=1`) |
| `SHORTR_LOG_LEVEL` | `info` | debug/info/warn/error |
| `SHORTR_LOG_FORMAT` | `json` | `json` \| `text` |
| `SHORTR_METRICS_ENABLED` | `true` | `/metrics` exposed; protect with `SHORTR_METRICS_TOKEN` or proxy ACL |
| `SHORTR_BACKUP_INTERVAL` | `24h` (sqlite only) | `0` disables. Uses `VACUUM INTO`, keeps last `SHORTR_BACKUP_KEEP=7` |
| `SHORTR_SHUTDOWN_TIMEOUT` | `15s` | |

Runtime-editable settings (admin UI, stored in `settings` table, override env where marked): blocked domains, registration mode, default redirect status, count bots, OIDC auto-create / auto-link toggles, site name, max links per user, fetch titles. Env value is the seed; DB value wins once set. Secrets (client secret, DSN, secret key) are **never** DB-stored or UI-editable.

---

## 7. Data Model

IDs: `TEXT` ULIDs generated in app (26 chars, sortable) — same on both DBs, no sequences to reconcile. Timestamps: `INTEGER` unix milliseconds on SQLite, `TIMESTAMPTZ` on Postgres (Store converts). Booleans: `INTEGER 0/1` on SQLite.

### 7.1 Tables

**users**
| column | type | notes |
|--------|------|-------|
| id | TEXT PK | ULID |
| email | TEXT UNIQUE NOT NULL | lower-cased, trimmed; citext semantics done in app |
| email_verified | BOOL | true if set by admin or verified OIDC claim |
| name | TEXT | display name |
| password_hash | TEXT NULL | NULL for OIDC-only accounts |
| role | TEXT | `admin` \| `user` |
| status | TEXT | `active` \| `disabled` |
| max_links | INTEGER NULL | per-user override |
| last_login_at | TS NULL | |
| created_at, updated_at | TS | |
| password_changed_at | TS NULL | sessions issued before this are invalid |

**oidc_identities**
| column | notes |
|--------|-------|
| id PK | |
| user_id FK → users ON DELETE CASCADE | |
| issuer TEXT NOT NULL | from token `iss` |
| subject TEXT NOT NULL | token `sub` |
| email TEXT | last seen |
| name TEXT | |
| raw_claims TEXT | JSON, for debugging/admin view; PII-aware (pruned to whitelisted keys) |
| created_at, last_login_at | |
| UNIQUE(issuer, subject) | |

**sessions**
| column | notes |
|--------|-------|
| id PK | random 32-byte token hashed (sha256) — DB stores hash only |
| user_id FK CASCADE | |
| created_at, expires_at, last_seen_at | |
| ip, user_agent | for "active sessions" page |
| csrf_token | random per session |

**api_keys**
| column | notes |
|--------|-------|
| id PK | |
| user_id FK CASCADE | |
| name TEXT | user label ("Android phone") |
| prefix TEXT | first 8 chars for display (`sk_live_ab12…`) |
| key_hash TEXT UNIQUE | sha256 of full key |
| scopes TEXT | JSON array: `links:read`, `links:write`, `stats:read`; default all |
| last_used_at, expires_at NULL, revoked_at NULL | |
| created_at | |

**links**
| column | notes |
|--------|-------|
| id PK | ULID |
| code TEXT UNIQUE NOT NULL | case-sensitive; see §8 |
| target_url TEXT NOT NULL | normalised |
| title TEXT | fetched or user-set |
| description TEXT | |
| user_id FK → users ON DELETE SET NULL | NULL = orphan (visible to admin) |
| redirect_status INTEGER | 301/302/307/308 |
| password_hash TEXT NULL | argon2id; NULL = none |
| expires_at TS NULL | |
| max_clicks INTEGER NULL | |
| click_count INTEGER default 0 | denormalised counter (fast list view) |
| last_click_at TS NULL | |
| status TEXT | `active` \| `disabled` |
| deleted_at TS NULL | soft delete → 410; hard purge after 30 d job (keeps code reserved meanwhile, prevents phishing reuse) |
| utm_source/medium/campaign/term/content TEXT NULL | appended at redirect if set |
| pass_query BOOL default true | forward `?x=y` from short URL to target |
| tags TEXT | JSON array of strings (≤10, each ≤32 chars) — `ponytail:` no join table; upgrade to `link_tags` if tag filtering gets slow |
| created_at, updated_at | |
| created_by_ip TEXT | audit |
| Indexes | `UNIQUE(code)`, `(user_id, created_at DESC)`, `(deleted_at)`, `(expires_at) WHERE expires_at IS NOT NULL` |

**clicks** (raw events; append-only)
| column | notes |
|--------|-------|
| id INTEGER PK autoincrement (sqlite) / BIGSERIAL (pg) | integer for compact index; ordering by time uses `ts` |
| link_id TEXT NOT NULL (no FK on purpose — hot path insert must not lock links) | |
| ts TS NOT NULL | |
| ip TEXT | per IP_MODE |
| ip_version INTEGER | 4/6 |
| country TEXT(2) | ISO from GeoIP or empty |
| region, city TEXT | only if City db configured |
| referrer TEXT | full referrer (≤512) |
| referrer_host TEXT | extracted host for grouping |
| user_agent TEXT | raw (≤512) |
| device TEXT | `desktop` \| `mobile` \| `tablet` \| `bot` \| `other` |
| os TEXT · os_version TEXT | |
| browser TEXT · browser_version TEXT | |
| is_bot BOOL | |
| lang TEXT | `Accept-Language` first tag |
| utm_source … utm_content | from the **short URL's** query if present |
| qs TEXT | raw query string on short URL (≤256) |
| Indexes | `(link_id, ts)`, `(ts)` |

**click_rollups_daily** (materialised aggregates for fast dashboards & retention)
| column | notes |
|--------|-------|
| link_id, day (DATE / integer yyyymmdd) | |
| dim TEXT | `total` \| `country` \| `device` \| `os` \| `browser` \| `referrer_host` |
| key TEXT | dimension value ('' for total) |
| clicks INTEGER · bots INTEGER · uniques INTEGER | uniques ≈ distinct ip per day (approximation acknowledged in UI) |
| PK (link_id, day, dim, key) | |

**settings** — `key TEXT PK, value TEXT (JSON), updated_at, updated_by`

**audit_log** — `id, ts, actor_user_id, actor_ip, action (link.create, user.disable, oidc.link, …), target_type, target_id, meta JSON`. Rotated with retention 365 d.

**schema_migrations** — `version INTEGER PK, applied_at`.

### 7.2 SQLite specifics
`PRAGMA journal_mode=WAL; synchronous=NORMAL; busy_timeout=5000; foreign_keys=ON; cache_size=-20000 (20 MB); temp_store=MEMORY; mmap_size=268435456`. Single writer connection pool (`SetMaxOpenConns(1)` on a dedicated *write* `*sql.DB`) + a read pool (`SetMaxOpenConns(8)`). This eliminates `SQLITE_BUSY` on writes entirely — the app serialises writes itself. `ponytail: two *sql.DB handles instead of a connection router; revisit if read/write split causes bugs.`

### 7.3 PostgreSQL specifics
`pgx` stdlib driver, pool 16, `statement_timeout=10s` set via DSN option, `application_name=shortr`. Uses `ON CONFLICT` upserts, `TIMESTAMPTZ`, `BIGSERIAL`, partial indexes. Advisory lock (`pg_advisory_lock(hash('shortr-migrate'))`) around migrations so two replicas can't migrate concurrently.

### 7.4 Dialect handling
Queries are written with `?` placeholders; `store.q(sql)` rewrites to `$n` for Postgres at init (cached). Divergences (upsert syntax, date bucketing, `RETURNING`) live in a small `dialect` struct with 6 functions. Migrations are separate per dialect. No ORM.

---

## 8. Short Code Design

- **Alphabet:** base58 default (`123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`) — removes visually ambiguous `0 O l I`. base62 selectable.
- **Length:** 7 chars default → 58^7 ≈ 2.2 × 10¹². Collision probability negligible; retry loop up to 5 times with length+1 on the 5th (never fails in practice; if it does, 503 with `CODE_EXHAUSTED`).
- **Generation:** `crypto/rand` bytes → rejection-sampled into alphabet (no modulo bias).
- **Custom alias:** 1–64 chars, `[A-Za-z0-9_-]`, cannot start/end with `-`/`_`, **case-sensitive stored** but **collision-checked case-insensitively** (`abc` and `ABC` cannot coexist — avoids confusion when spoken). Store a `code_lower` generated column with unique index? → Decision: `UNIQUE INDEX ON links(lower(code))` (both DBs support expression indexes) plus `UNIQUE(code)`. Lookup is exact-match on `code` first; if miss, one case-insensitive lookup, redirect with the canonical code (302 to canonical short URL? No — just resolve; avoids double hop). `ponytail: case-insensitive fallback is one extra query on miss only.`
- **Reserved words** (cannot be aliases; also cannot be generated because they contain excluded chars or are checked): `api, app, auth, admin, assets, static, metrics, health, healthz, ready, robots.txt, favicon.ico, sitemap.xml, login, logout, setup, oidc, qr, s, u, l, p, docs, .well-known` and any code beginning with `_` or `.`.
- **Profanity list:** small embedded list (~200 words) checked on auto-generated codes only (regenerate) — not on aliases (user's choice).
- **Routing precedence:** `/api/`, `/app/`, `/auth/`, `/assets/`, `/health*`, `/metrics`, `/` (landing → redirect to `/app`) are registered first; `GET /{code}` is the catch-all. Codes never contain `/`, so `/{code}/anything` → 404.
- **Deleted links** keep their code reserved for 30 days (soft delete) then are purged; the code becomes reusable. Admin can force-purge.
- **Vanity length:** users may request a length (4–16) at create time for auto-generated codes; min 4 to keep enumeration expensive.

---

## 9. Redirect Hot Path

### 9.1 Cache
- Sharded LRU: 16 shards by `fnv32(code) % 16`, each `map[string]*entry` + `container/list`, mutex per shard. Capacity default 50,000 entries (~25 MB). Configurable.
- Entry holds the fully resolved struct needed for redirect decisions (target, status, expires_at, max_clicks, password flag, disabled/deleted flags, pass_query, utm) — no DB touch on hit.
- TTL 5 min (safety net for external DB edits); write-through invalidation on every link mutation via service layer, so TTL rarely matters. With Postgres and multiple replicas (not officially supported in v1) the TTL bounds staleness.
- **Negative cache:** unknown codes cached for 60 s (`entry.missing=true`) — defends against scanning storms hitting the DB.
- **Click-expiry check:** `max_clicks` compares against an in-memory atomic counter seeded from DB on cache load and incremented on hit; the DB `click_count` is updated by the batch writer. Off-by-a-few at the boundary is acceptable and documented (§24).

### 9.2 Handler pseudocode
```
code := r.PathValue("code")
if !validCode(code) → 404 page
l := cache.Get(code) ?? store.GetLinkByCode(code) (also lower(code) fallback) → cache.Put
switch:
  nil / deleted        → 404 (deleted → 410)
  disabled             → 404 page "This link has been disabled"
  expires_at < now     → 410 page "expired"
  max_clicks reached   → 410 page "expired"
  password != nil      → if valid cookie `lp_<code>` (HMAC of link id + pw hash version, 1 h) → continue; else 200 password form (POST /{code} verifies, rate-limited 5/min/IP, sets cookie, 303 → GET /{code})
target := l.target
if l.pass_query && r.URL.RawQuery != "" → merge query (short URL params win over duplicates in target? No: target's own params kept, short URL params appended; utm_* in short URL override link-level utm)
if link utm set → append missing utm_* keys
w.Header: Location, Cache-Control: no-store, X-Robots-Tag: noindex
w.WriteHeader(l.status)
emit click (non-blocking) unless HEAD (HEAD is served but never counted)
```

### 9.3 Method handling
- `GET` → redirect + count. `HEAD` → redirect, no count (link checkers). `POST /{code}` → password submit only; otherwise 405. Others → 405.
- `OPTIONS` → 204 with `Allow` (some extensions preflight).

---

## 10. Analytics & Click Recording

### 10.1 Event pipeline
1. Handler builds `ClickEvent{linkID, ts, rawIP, ua, referrer, lang, qs, utm}` — **no parsing in the handler** (UA parse & GeoIP happen in the worker to keep the hot path minimal).
2. Buffered channel, capacity 10,000. Send is non-blocking; on full, the event is dropped and `shortr_clicks_dropped_total` increments + a rate-limited warn log. Redirect is never delayed.
3. Writer goroutine: pulls events, enriches (UA parse, bot detection, GeoIP, IP anonymisation), appends to batch. Flush when batch ≥ 500 or 250 ms elapsed. One transaction: multi-row `INSERT INTO clicks`, plus `UPDATE links SET click_count = click_count + ?, last_click_at = ? WHERE id = ?` aggregated per link in the batch (only non-bot unless `COUNT_BOTS`).
4. On DB error: retry the batch 3× with backoff (100 ms, 500 ms, 2 s); if still failing, write the batch as JSON lines to `$DATA_DIR/clicks-spool/<ts>.jsonl` and continue. A recovery job replays spool files when the DB is healthy. So clicks survive DB outages (Postgres restart) and disk-full is the only true loss.
5. Graceful shutdown: server stops accepting, channel is closed, writer drains and flushes (bounded by `SHUTDOWN_TIMEOUT`); remainder spooled to disk.

### 10.2 What is recorded
| Field | Source | Notes |
|-------|--------|-------|
| IP | real IP after trusted-proxy resolution | stored per `IP_MODE`; default anonymised |
| Country/region/city | GeoIP mmdb (optional) | empty if not configured; lookup uses the **full** IP before anonymisation |
| Device / OS / browser | UA parse | `bot` if UA matches bot list (Googlebot, bingbot, facebookexternalhit, Twitterbot, Slackbot, WhatsApp, TelegramBot, Discordbot, LinkedInBot, curl, wget, python-requests, Go-http-client, HeadlessChrome, …) or missing UA |
| Referrer | `Referer` header | host extracted; `direct` if empty; the short domain itself is ignored |
| Language | `Accept-Language` first tag | |
| UTM | short URL query | |

**Link-preview fetchers** (WhatsApp, Slack, iMessage, Telegram) hit the short URL when a link is pasted. They are classified as bots and not counted by default — this is the #1 source of "phantom clicks" in naive shorteners.

### 10.3 Uniques
"Unique visitors" per day = `COUNT(DISTINCT ip)` in raw clicks for the day (under `anonymize` mode this is a /24 approximation; the UI labels it "approx. unique"). No cookies are set on visitors (privacy by design; no cookie banner needed on the redirect path).

### 10.4 Rollups & retention
- Hourly job aggregates yesterday's (and any un-rolled days') raw clicks into `click_rollups_daily` (idempotent upsert, tracked via `settings.rollup_watermark`).
- Dashboards for ranges > 7 days query rollups; ≤ 7 days query raw for hour granularity.
- Retention job deletes raw clicks older than `CLICK_RETENTION_DAYS` in chunks of 5,000 rows (avoid long locks), only for days already rolled up.
- Export: `GET /api/v1/links/{id}/clicks/export?format=csv|json` streams raw clicks (chunked, no memory buildup), with CSV formula-injection protection (prefix `'` for cells starting with `= + - @`).

### 10.5 Queries exposed
- Time series (clicks, uniques, bots) with bucket = hour/day/week/month, tz from the user (`?tz=Asia/Kolkata`), computed in Go for SQLite (bucket by ts integer division after tz offset is tricky — decision: **bucket in SQL by UTC day, shift in UI for day+ granularity; hour granularity uses `ts` and the UI's tz**). Documented limitation §25.
- Breakdown by country / device / os / browser / referrer_host with top-N + "other".
- Global dashboard (user scope or admin scope): totals, top links, recent clicks (live feed, polled every 10 s).

---

## 11. Authentication & Authorization

### 11.1 First run
- If `users` is empty: all `/app` routes redirect to `/app/setup`. `POST /auth/setup` creates the first admin (email, name, password). Rate-limited, only works while user count = 0 (checked in the same transaction — two racing setups: second gets 409). If OIDC is configured, setup page also offers "Sign in with SSO to become admin" (first OIDC user becomes admin only during setup state).
- Alternatively `shortr admin create --email … --password …` CLI, or `SHORTR_ADMIN_EMAIL`/`SHORTR_ADMIN_PASSWORD` env seeding on first boot (useful in Docker).

### 11.2 Local login
- `POST /auth/login {email, password}`; argon2id (t=3, m=64 MB, p=1 — tuneable; ~50 ms) verify. Constant-time behaviour on unknown email (hash a dummy).
- Lockout: after 10 failed attempts per email **or** per IP in 15 min → 429 for 15 min (stored in memory; `ponytail: in-memory lockout resets on restart; move to DB if that matters`).
- Session: 32 random bytes → base64url token in cookie `shortr_session` (`HttpOnly; Secure (when https); SameSite=Lax; Path=/`). DB stores `sha256(token)`. Sliding expiry: `last_seen_at` bumped at most once per 5 min. Absolute max 90 d.
- Logout deletes the row. Password change deletes all other sessions. Admin "disable user" deletes all sessions.
- Password policy: 10–128 chars, no composition rules (NIST), checked against a small embedded list of top 10k breached passwords (`ponytail: embedded list, ~80 KB; HIBP k-anonymity API would be a 3rd-party call so no`).

### 11.3 API keys (for Android/extension/scripts)
- Format `sk_<8-char prefix>_<32 random base62>`; shown **once** at creation. Stored hashed.
- `Authorization: Bearer sk_…`. On use: `last_used_at` updated at most once/min.
- Scopes: `links:read links:write stats:read`. Default all three; `admin:*` only for admin users' keys and never by default.
- Not subject to CSRF (no cookie). Subject to `RATE_LIMIT_API` per key.
- Optional expiry; revocation immediate (cache of hashes invalidated).

### 11.4 CSRF
- Cookie sessions: every mutating `/api/*` and `/auth/*` request must carry `X-CSRF-Token` equal to the session's `csrf_token` (exposed via `GET /api/v1/me`). Additionally `Origin`/`Referer` must match `BASE_URL` origin when present. `SameSite=Lax` is defence-in-depth, not the only control.
- Bearer-auth requests skip CSRF.

### 11.5 Authorization
| Action | user | admin |
|--------|------|-------|
| CRUD own links, own stats, own API keys, own sessions, own OIDC links | ✔ | ✔ |
| View/edit any link, reassign owner | | ✔ |
| Manage users (create, disable, reset password, role) | | ✔ |
| Settings, audit log, global stats | | ✔ |
| Delete own account | ✔ (unless last admin) | ✔ |

Ownership checks are done in the service layer by `(link.user_id == actor.id || actor.role == admin)` — a single `authorize()` helper, not per-handler ad hoc checks. Links with `user_id NULL` (orphaned) are admin-only.

---

## 12. OIDC (OpenID Connect)

### 12.1 Flow (Authorization Code + PKCE, confidential or public client)
1. `GET /auth/oidc/start?next=/app/links` → generate `state` (random 32 B) + `nonce` + PKCE `code_verifier`; store `{state, nonce, verifier, next, link_user_id?, exp: now+10m}` in a **signed, encrypted cookie** (`shortr_oidc`, HMAC via SECRET_KEY, `SameSite=Lax`, 10 min) — `ponytail: cookie instead of DB row; stateless & multi-tab safe`. Redirect to provider `authorization_endpoint` with `response_type=code`, `scope`, `state`, `nonce`, `code_challenge (S256)`, `redirect_uri = BASE_URL/auth/oidc/callback`.
2. `GET /auth/oidc/callback?code&state` → verify cookie present & `state` matches → exchange code (`token_endpoint`, with `code_verifier`) → verify ID token via go-oidc (signature via JWKS, `iss`, `aud`, `exp`, `nonce`) → extract claims: `sub` (required), `email`, `email_verified`, `name`/`preferred_username`, `groups`.
3. If `REQUIRE_EMAIL_VERIFIED` and `email_verified != true` → error page `OIDC_EMAIL_UNVERIFIED` (unless email absent and auto-create is off and a linked identity exists — then email isn't needed).
4. Resolution order:
   - **Identity exists** `(iss, sub)` → log in that user (unless user disabled → error). Update `email`, `name`, `last_login_at` on identity. If the identity's user is a **different** user than a currently logged-in session's user (link flow), error `OIDC_ALREADY_LINKED`.
   - **Link flow** (cookie has `link_user_id` = the currently logged-in user who clicked "Link SSO account" in settings, and session still valid & same user) → create identity for that user. Requires re-auth? Decision: the link button requires the user to have logged in within the last 10 min (`sudo mode`), otherwise re-prompt password. Audit `oidc.link`.
   - **No identity, `AUTO_LINK_BY_EMAIL=true`, local user with same email exists, email verified** → create identity, log in. Audit `oidc.autolink`.
   - **No identity, local user with same email exists, auto-link off** → error page `OIDC_ACCOUNT_EXISTS`: "An account with this email already exists. Sign in with your password, then link SSO from Settings → Security." (with a button to local login preserving `next`).
   - **No identity, no user, `AUTO_CREATE=true`** → create user (`email`, `name`, `password_hash=NULL`, role=`user` or `admin` if `groups` contains `OIDC_ADMIN_GROUP`; `email_verified=true`), create identity, log in. Audit `user.create(oidc)`. Respects `MAX_USERS` (none in v1).
   - **No identity, no user, auto-create off** → error page `OIDC_NO_ACCOUNT`: "No account for you. Ask an admin to invite you."
5. Create session, clear `shortr_oidc` cookie, `303 → next` (validated: must be a relative path starting with `/app`, no `//`, no scheme — open-redirect defence).

### 12.2 Account linking UI (Settings → Security)
- Shows linked identities (provider name, email, linked date). Buttons: **Link SSO account**, **Unlink**.
- **Unlink** rule: allowed only if the user has a password set **or** another identity remains (never strand an account). Requires sudo mode. Audit.
- **Set a password** for OIDC-only accounts (so they can unlink or use API from CLI): requires current session sudo (re-auth via OIDC prompt = `prompt=login` round-trip, or just fresh login within 10 min).
- Admin can unlink any user's identity, and can "Convert to OIDC-only" (removes password).

### 12.3 Group → role sync
- On every OIDC login, if `OIDC_ADMIN_GROUP` is set: role = admin if group present else user — **unless** user has `role_locked=true` (admin-set flag so a local super-admin isn't demoted by IdP). `ponytail: role_locked column instead of role-source tracking.`

### 12.4 Provider quirks handled
- Discovery cached; JWKS refreshed by go-oidc on unknown `kid`.
- Providers that put email only in `userinfo` (some Keycloak configs): if `email` claim absent from ID token, call `userinfo_endpoint` once.
- Azure AD `tid`/`oid`: `sub` is pairwise per app — fine since we key by `(iss, sub)`.
- Google: `email_verified` present; `hd` claim optionally enforced via `OIDC_ALLOWED_DOMAINS` (comma list) — applied to email domain for all providers.
- Clock skew: go-oidc tolerates via `exp` with `now` — we set a 60 s leeway.
- Logout: local session only. RP-initiated logout (`end_session_endpoint`) is optional: if discovered and `OIDC_SLO=true`, `/auth/logout` redirects there with `post_logout_redirect_uri`. Back-channel logout: not in v1.
- Multiple providers: **v1 supports exactly one** (schema supports many via `issuer`; config supports one). Documented limitation.

---

## 13. HTTP API (v1)

Base: `/api/v1`. JSON only (`Content-Type: application/json; charset=utf-8`). All timestamps RFC 3339 UTC. IDs are ULIDs.

### 13.1 Envelope
Success: the resource or `{ "items": [...], "next_cursor": "..." , "total": n }` for lists.
Error:
```json
{ "error": { "code": "VALIDATION_FAILED", "message": "target_url must be an http(s) URL", "fields": { "target_url": "must be an http(s) URL" }, "request_id": "01J..." } }
```

### 13.2 Endpoints

**Auth / session**
| Method & Path | Auth | Description |
|---|---|---|
| `GET /auth/status` | none | `{setup_required, oidc_enabled, oidc_display_name, local_login, registration}` |
| `POST /auth/setup` | none (only when no users) | Create first admin |
| `POST /auth/login` | none | `{email,password}` → sets cookie; returns user |
| `POST /auth/logout` | session | |
| `POST /auth/register` | none (if registration open) | |
| `GET /auth/oidc/start` | none | Redirect to IdP |
| `GET /auth/oidc/callback` | none | |
| `GET /auth/oidc/link` | session (sudo) | Start link flow |
| `POST /auth/sudo` | session | Re-enter password → refresh sudo window |

**Me**
| | | |
|---|---|---|
| `GET /api/v1/me` | any | user + `csrf_token` + capabilities |
| `PATCH /api/v1/me` | any | name, email (requires password) |
| `PUT /api/v1/me/password` | session | `{current?, new}`; current optional if none set |
| `GET/DELETE /api/v1/me/sessions[/{id}]` | | active sessions |
| `GET /api/v1/me/identities` · `DELETE /api/v1/me/identities/{id}` | | OIDC links |
| `DELETE /api/v1/me` | session (sudo) | delete account; links → orphaned or deleted (`?links=delete|keep`) |

**Links**
| | | |
|---|---|---|
| `POST /api/v1/links` | `links:write` | body below |
| `GET /api/v1/links` | `links:read` | `?q=&tag=&status=&sort=created_at|clicks|title&order=&cursor=&limit=` (≤100) |
| `GET /api/v1/links/{id}` | | |
| `PATCH /api/v1/links/{id}` | | any mutable field; `code` change allowed (old code → 404 immediately, not kept) |
| `DELETE /api/v1/links/{id}` | | soft delete |
| `POST /api/v1/links/{id}/restore` | | within 30 d |
| `POST /api/v1/links/bulk` | | `{items:[…≤100]}` → per-item result (207-style array) |
| `GET /api/v1/links/{id}/qr.png?size=256&fg=&bg=` | | PNG QR, also `.svg` |
| `GET /api/v1/links/check?code=xyz` | | alias availability |
| `GET /api/v1/links/{id}/stats?from&to&bucket&tz` | `stats:read` | series + breakdowns |
| `GET /api/v1/links/{id}/clicks?cursor&limit` | | raw click list |
| `GET /api/v1/links/{id}/clicks/export?format=csv` | | streamed |
| `POST /api/v1/links/preview` | | `{url}` → `{title, final_url}` (only when FETCH_TITLES) |

Create body:
```json
{ "target_url": "https://…", "code": "optional-alias", "length": 7, "title": "", "description": "",
  "redirect_status": 302, "password": "", "expires_at": null, "max_clicks": null,
  "tags": ["a"], "pass_query": true, "utm": { "source": "", "medium": "", "campaign": "", "term": "", "content": "" } }
```
Response adds `id, short_url, click_count, created_at, updated_at, status, has_password`.

**Stats**
| | | |
|---|---|---|
| `GET /api/v1/stats/overview?from&to` | any | totals, top links, series (user scope; `?scope=all` admin) |
| `GET /api/v1/stats/recent?limit=50` | | live feed |

**API keys** — `GET/POST /api/v1/apikeys`, `DELETE /api/v1/apikeys/{id}` (session only, never via API key: keys cannot mint keys).

**Admin** — `GET/POST /api/v1/users`, `GET/PATCH/DELETE /api/v1/users/{id}`, `POST /api/v1/users/{id}/reset-password` (returns one-time password), `DELETE /api/v1/users/{id}/sessions`, `DELETE /api/v1/users/{id}/identities/{iid}`, `GET/PUT /api/v1/settings`, `GET /api/v1/audit?cursor`, `GET /api/v1/admin/links` (all links), `POST /api/v1/admin/links/{id}/purge`, `GET /api/v1/admin/system` (version, db, cache stats, queue depth, uptime), `POST /api/v1/admin/backup` (sqlite).

**Ops** — `GET /healthz` (liveness: 200 always), `GET /readyz` (DB ping ≤ 2 s, migrations current, click queue < 90 % → 200 else 503 with reasons), `GET /metrics`, `GET /version`.

### 13.3 Conventions
- Cursor pagination (opaque base64 of `(sort_value, id)`), never offset — stable under inserts.
- `PATCH` = partial update; absent fields untouched; `null` explicitly clears nullable fields.
- Idempotency: `POST /links` accepts `Idempotency-Key` header (stored 24 h in memory keyed by user) → same response on replay. `ponytail: in-memory; DB table if multi-replica`.
- `ETag`/`If-Match` on link PATCH to prevent lost updates from two tabs (`updated_at`-based weak ETag; mismatch → 412).
- CORS: `/api/*` allows origins from `SHORTR_CORS_ORIGINS` (default: none besides same-origin) plus `chrome-extension://<id>` / `moz-extension://…` entries the admin whitelists. Credentials not allowed cross-origin — extensions use API keys.
- OpenAPI 3.1 spec hand-maintained at `docs/openapi.yaml`, served at `/api/v1/openapi.json`, with Scalar/Swagger UI at `/app/docs`.

---

## 14. Frontend (React) Design

### 14.1 Routes (`/app` base)
```
/app/setup                      first-run wizard (only when setup_required)
/app/login                      local form + SSO button
/app                            Dashboard (overview stats, recent clicks, quick-shorten box)
/app/links                      Links table (search, filters, bulk actions)
/app/links/new                  Create (also as Dialog from anywhere via ⌘K / "+")
/app/links/:id                  Link detail: analytics tabs, edit, QR, share
/app/settings/profile
/app/settings/security          password, sessions, SSO links, 2FA (future)
/app/settings/api-keys
/app/admin/users · /app/admin/users/:id
/app/admin/settings
/app/admin/audit
/app/admin/system
/app/docs                       API reference
```
Guards: `RequireAuth`, `RequireAdmin`, `RequireSetup`. Unknown → 404 page in-app.

### 14.2 Data layer
- `lib/api.ts`: `apiFetch<T>(path, init)` adds `X-CSRF-Token`, parses envelope, throws `ApiError{code, fields}`; 401 → redirect to login preserving `next`; 403 → toast.
- Query keys: `['links', filters]`, `['link', id]`, `['stats', id, range]`, `['me']`. Mutations invalidate precisely; optimistic delete/toggle in the table.
- Types generated from OpenAPI (`openapi-typescript`) at build → no drift.

### 14.3 Performance budget
- Initial JS ≤ 250 KB gzip (route-level code splitting: charts & admin lazy).
- Fonts: system stack (`Inter` optional self-hosted, no Google Fonts call — "no 3rd-party").
- All assets hashed & served with `Cache-Control: public, max-age=31536000, immutable`; `index.html` `no-cache`.

---

## 15. UI / UX Specification

### 15.1 Design language
- **Aesthetic:** clean "product" look — neutral zinc palette, one accent (indigo `hsl(243 75% 59%)`), 8-pt spacing scale, 12 px radius, subtle borders over heavy shadows, generous whitespace. Dark mode first-class (system default + toggle, persisted).
- **Typography:** `Inter` (self-hosted, variable) / system fallback; tabular numbers for stats; monospace (`JetBrains Mono` fallback `ui-monospace`) for codes/URLs.
- **Layout:** left sidebar (collapsible to icons; sheet on mobile), top bar with global search / command palette (⌘K), content max-width 1200 px. Responsive breakpoints: 640 / 768 / 1024 / 1280.
- **Motion:** 150–200 ms ease-out for state changes; `prefers-reduced-motion` respected; no decorative animation.

### 15.2 Key screens

**Dashboard**
- Hero "Shorten a link" input (paste-and-go: paste → auto-create with defaults → result card with copy button, QR, "Customize" expands options). This is the 80 % use case and must take ≤ 2 interactions.
- Stat tiles: Total clicks (range), Unique visitors, Active links, Top referrer — with sparkline & delta vs previous period.
- Clicks chart (area, range picker 24h/7d/30d/90d/custom).
- Recent activity feed (link, country flag, device icon, relative time), polling 10 s, pauses when tab hidden.

**Links table**
- Columns: short (code, copy on click, hover shows full), target (favicon from `/api/v1/links/{id}/favicon`? — no, that's an outbound fetch; use a generic globe icon; `ponytail: no favicon proxy`), title, clicks (sparkline last 7 d), created, status badge, tags, actions (⋯ menu: edit, QR, stats, disable, delete).
- Search debounced 300 ms (code, title, target). Filters: status, tag, date. Sort by clicks/created.
- Row click → detail. Multi-select → bulk disable/delete/tag. Keyboard: `j/k` navigate, `c` copy, `e` edit, `/` focus search.
- Empty state with illustration and primary CTA. Loading: skeleton rows. Errors: inline retry.

**Create/Edit dialog**
- Fields ordered by frequency: Target URL (autofocus, validates on blur, shows fetched title), Custom alias (inline availability check with 400 ms debounce, shows `short.domain/`, green check / red x), then "More options" accordion: title, tags (combobox with create), expiry (date-time picker + presets 1h/1d/7d/30d), click limit, password (with show/hide & generate), redirect type (radio with one-line explanation each), UTM builder, pass-through query toggle.
- Submit → toast with "Copied to clipboard" (auto-copy on create is a user preference, default on).

**Link detail**
- Header: short URL (copy), target (truncated middle, open in new tab icon), status, owner (admin).
- Tabs: **Overview** (series, totals), **Audience** (countries map-less list with flags & bars, devices donut, OS/browser bars), **Referrers** (table), **Clicks** (raw table, export), **Settings** (edit form), **QR** (preview, size, colors, PNG/SVG download).
- Danger zone: disable, delete (confirm dialog typed `delete` not required — soft delete is restorable; undo toast 8 s).

**Login**
- Centered card; SSO button prominent if enabled; local form below (or hidden per config). Error messages generic ("Invalid email or password"). Setup wizard reuses layout with 3 steps: admin account → site basics (name, base URL confirmation with proxy detection hint) → done.

**Settings/Security**
- Password section (set/change), Active sessions (device, IP, last seen, revoke, "this device" label), Connected accounts (SSO: link/unlink with rules explained inline), API keys page (create → one-time reveal with copy & warning; table with prefix, scopes, last used, revoke).

**Admin**
- Users table (search, role badge, status, links count, last login; actions: edit, disable, reset password, impersonate? — **no impersonation in v1**), user detail with their links & identities.
- Settings form grouped: General, Links defaults, Privacy (IP mode explained), Registration & SSO toggles, Blocked domains (textarea), Retention.
- System page: version, DB driver & size, cache hit ratio, queue depth, dropped clicks, uptime, backup now button, last backup.
- Audit log table with filters.

**Public pages (Go templates, no JS required)**
- 404 (not found), 410 (expired/removed), disabled, password prompt (with error state, rate-limit message). Minimal, branded with site name, dark-mode via `prefers-color-scheme`, no external assets. Include `<meta name="robots" content="noindex">`.

### 15.3 UX rules
- Every destructive action is either undoable (soft delete + undo toast) or confirmed.
- Never block on analytics loading; skeletons per widget.
- Copy actions give immediate feedback; short URLs displayed without scheme (`sho.rt/abc`), copied with scheme.
- Forms show server field errors next to the field (`fields` map), never only a toast.
- Focus management in dialogs (Radix handles), visible focus rings, all icon buttons have `aria-label`, color contrast ≥ 4.5:1, charts have data tables toggle for screen readers.
- Time shown in user's local tz with UTC tooltip.
- Offline/network error banner with retry; TanStack Query retries 2× with backoff on 5xx/network, never on 4xx.
- Mobile: bottom tab bar for Dashboard / Links / New / Settings; tables collapse to cards.

---

## 16. Validation Rules

All enforced server-side; mirrored in zod on the client.

| Field | Rule |
|-------|------|
| `target_url` | Required. Trim. Length 1–`MAX_URL_LENGTH` bytes. Must parse with `net/url`; scheme `http` or `https` only (reject `javascript:`, `data:`, `file:`, `ftp:`). Must have a host. Host validated: DNS label rules (IDN allowed → stored as punycode via `golang.org/x/net/idna`? — that's another dep; **decision: accept and store as given; `net/url` handles it; punycode conversion done by browser at redirect**). Reject userinfo (`user:pass@`) — phishing vector. Reject host equal to our own base host (self-loop) unless admin. Reject literal IPs in private/loopback/link-local ranges and `localhost`, `*.local`, `*.internal` unless `ALLOW_PRIVATE_TARGETS`. Blocked domains list (exact or suffix match). Fragment preserved. Whitespace inside → reject (no auto-encoding; ambiguity). Normalise: lowercase scheme & host, remove default port, keep path/query verbatim. |
| `code` (alias) | Optional. 1–64 chars `[A-Za-z0-9_-]`, not starting/ending with `-`/`_`, not reserved, not existing (case-insensitive incl. soft-deleted within reservation window). Not all digits? — allowed. |
| `length` | 4–16, ignored if `code` given |
| `title` | ≤ 200 chars, control chars stripped |
| `description` | ≤ 1000 chars |
| `redirect_status` | ∈ {301, 302, 307, 308} |
| `password` | 1–128 chars; empty string in PATCH = remove password |
| `expires_at` | RFC 3339; must be > now + 1 min; ≤ 100 years |
| `max_clicks` | 1–2,147,483,647; null clears |
| `tags` | ≤ 10 items, each 1–32 chars, `[\p{L}\p{N} _-]`, deduped case-insensitively, stored trimmed |
| `utm.*` | each ≤ 255 chars, no CR/LF |
| `pass_query` | bool |
| `email` | Trim, lowercase, ≤ 254, `net/mail.ParseAddress` + must contain `@` and a dot in domain; no display-name part |
| `password` (user) | 10–128 chars, not in breached list, not equal to email |
| `name` | 1–100 chars, control chars stripped |
| `apikey.name` | 1–64 |
| `apikey.scopes` | subset of known |
| `apikey.expires_at` | > now |
| Pagination `limit` | 1–100 default 25; invalid → 400 |
| Date ranges | `from ≤ to`, span ≤ 2 years, both RFC 3339 or `YYYY-MM-DD` |
| `tz` | valid IANA name (`time.LoadLocation`; requires embedded tzdata → `import _ "time/tzdata"`) |
| `next` (redirect after login) | relative path, starts with `/app`, no `\`, no `//`, ≤ 512 |
| JSON bodies | ≤ 64 KB; unknown fields rejected; bulk ≤ 1 MB / 100 items |
| Query strings on short URLs | ≤ 2 KB total after merge, else redirect without merge |
| Headers | `User-Agent`, `Referer` truncated to 512 bytes before storage; invalid UTF-8 replaced |

---

## 17. Error Handling & Error Codes

### 17.1 Policy
- Handlers return `error`; a single `respond.Error(w, err)` maps typed errors → HTTP. Unknown errors → 500 with generic message + `request_id`; full error and stack logged once.
- `panic` in a handler → recovered by middleware → 500 + log with stack; the process never dies from a request. Panics in background goroutines are recovered and logged; the job is rescheduled.
- Never leak SQL, file paths, or stack traces to clients.
- DB errors classified: unique violation → 409; FK → 409/404; busy/timeout → 503 with `Retry-After: 2`; connection → 503 and `readyz` goes red.
- Context cancellation (client disconnected) → no response, log at debug.
- Timeouts: `ReadHeaderTimeout 5s`, `ReadTimeout 15s`, `WriteTimeout 30s` (60 s for exports via `http.ResponseController.SetWriteDeadline`), `IdleTimeout 120s`, per-request DB context 10 s.

### 17.2 Error codes
| HTTP | code | when |
|---|---|---|
| 400 | `BAD_REQUEST` | malformed JSON, bad query params |
| 400 | `VALIDATION_FAILED` | field errors (see `fields`) |
| 401 | `UNAUTHENTICATED` | no/invalid session or key |
| 401 | `SESSION_EXPIRED` | |
| 403 | `FORBIDDEN` | not owner / not admin / scope missing |
| 403 | `CSRF_FAILED` | |
| 403 | `SUDO_REQUIRED` | sensitive action needs recent auth |
| 403 | `USER_DISABLED` | |
| 404 | `NOT_FOUND` | |
| 409 | `CODE_TAKEN` | alias in use |
| 409 | `EMAIL_TAKEN` | |
| 409 | `SETUP_DONE` | setup called after first user |
| 409 | `OIDC_ALREADY_LINKED` / `OIDC_ACCOUNT_EXISTS` / `OIDC_NO_ACCOUNT` / `OIDC_EMAIL_UNVERIFIED` / `OIDC_DOMAIN_NOT_ALLOWED` | see §12 |
| 409 | `LAST_ADMIN` | can't demote/delete/disable the last admin |
| 409 | `CANNOT_UNLINK` | would strand account |
| 410 | `LINK_GONE` | deleted/expired (API view) |
| 412 | `PRECONDITION_FAILED` | ETag mismatch |
| 413 | `PAYLOAD_TOO_LARGE` | |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | |
| 422 | `TARGET_BLOCKED` | blocked domain / private target |
| 422 | `LINK_LIMIT_REACHED` | per-user max |
| 429 | `RATE_LIMITED` | with `Retry-After` |
| 429 | `LOCKED_OUT` | login lockout |
| 500 | `INTERNAL` | |
| 503 | `DB_UNAVAILABLE` / `CODE_EXHAUSTED` / `SHUTTING_DOWN` | |

---

## 18. Security

| Area | Control |
|------|---------|
| Transport | TLS terminated at NPM/proxy; app also supports `SHORTR_TLS_CERT/KEY` for direct TLS (stdlib) and `SHORTR_AUTOCERT=false` (no ACME in v1 — proxies do it better) |
| Headers (app pages) | `Content-Security-Policy: default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'` · `X-Content-Type-Options: nosniff` · `Referrer-Policy: strict-origin-when-cross-origin` · `X-Frame-Options: DENY` · `Permissions-Policy: camera=(), microphone=(), geolocation=()` · `Strict-Transport-Security` only when https |
| Headers (redirects) | `X-Robots-Tag: noindex`, `Cache-Control: no-store`, `Referrer-Policy` configurable |
| Passwords | argon2id, per-user salt, params versioned in hash string for future upgrades; rehash on login if params outdated |
| Sessions | random 256-bit, hashed at rest, HttpOnly, SameSite=Lax, Secure, rotated on login/privilege change |
| CSRF | token + Origin check (§11.4) |
| Open redirect | `next` validated; the short-link redirect itself is by design an open redirect to a **stored, validated** URL — mitigations: blocked domains, private-IP block, no `javascript:`; admin can disable/purge; abuse report link on public pages (`mailto:` admin, configurable) |
| SSRF (title fetch) | Resolve host; reject private/loopback/link-local/multicast/unique-local & IPv4-mapped IPv6; custom `DialContext` re-checks the resolved IP (DNS rebinding); follow ≤ 3 redirects re-checking each; 3 s timeout; 256 KB read cap; only `text/html`; no cookies; fixed UA |
| Enumeration | 7-char base58 random codes; negative cache; per-IP rate limit; no "does alias exist" without auth (the `check` endpoint is authenticated) |
| Rate limits | per IP / key / email (§19) |
| Injection | `database/sql` params only; `html/template` autoescape; JSON encoder; CSV escaping |
| Secrets | never logged; config print redacts; `secret.key` file 0600; API keys/session tokens hashed |
| Dependencies | `govulncheck` in CI; `go.sum` pinned; frontend `npm audit` |
| Container | non-root UID 65532, read-only root FS, `/data` volume only, no shell (distroless) |
| Uploads | none in v1 (no attack surface) |
| Admin surface | `/metrics` token-protected; `/readyz` reveals no details unless `?verbose` with admin |
| Audit | all auth events and admin actions logged with actor + IP |
| Account safety | last-admin protection; disabling a user kills sessions & API keys; deleting a user requires choice for links |
| Link passwords | argon2id (lighter params: t=1, m=16 MB), 5 attempts/min/IP/link, cookie scoped to path `/{code}`? — cookies can't be path-precise for `/abc` vs `/abcd` reliably; use cookie name `lp_<code>` with HMAC payload, `Path=/` |
| Time-based attacks | constant-time compares for tokens (`subtle.ConstantTimeCompare`) |
| Backups | sqlite `VACUUM INTO` (consistent snapshot), file 0600, retention |

---

## 19. Traffic Handling, Rate Limiting & Performance

### 19.1 Targets (single node, 2 vCPU, SQLite)
| Metric | Target |
|--------|--------|
| Redirect throughput (cache hit) | ≥ 10,000 req/s |
| Redirect p99 latency (cache hit) | < 5 ms (excluding network) |
| Redirect p99 (cache miss) | < 20 ms |
| Click write throughput | ≥ 5,000 clicks/s sustained (batched) |
| API list 25 links | < 30 ms |
| Stats 30-day (rollups) | < 50 ms |
| Memory (idle / 50k cached links / 10k queued clicks) | < 40 MB / < 80 MB / +10 MB |
| Cold start | < 300 ms |

Verified in Phase 6 with `bombardier`/`hey` and a Go benchmark test; numbers get written into README.

### 19.2 Rate limiting
- Token bucket per key: `{capacity, refill/s}` parsed from `N/Ds` config. Map with last-access; GC every minute removes idle > 10 min. Memory bound: 1M keys ≈ 100 MB worst case; cap keys at 500k with random eviction. `ponytail: in-memory per node; shared limiter only needed for multi-replica.`
- Keys: redirect → client IP (IPv6 by /64 to avoid trivial bypass); API → user id or API key id (fallback IP); auth endpoints → IP **and** email.
- Response: `429`, `Retry-After`, `X-RateLimit-Limit/Remaining/Reset` on API routes only (not on redirects — avoid header overhead).
- Allowlist CIDRs (`SHORTR_RATE_LIMIT_EXEMPT`) for monitoring systems.

### 19.3 Performance techniques
- Cache (§9.1); negative cache; prepared statements (`database/sql` caches per conn); batch click inserts; denormalised `click_count`; rollups; cursor pagination; covering indexes; streaming exports; `sync.Pool` for click events; avoid allocations on hot path (pre-built `Location` when no query merge needed).
- Static assets served from `embed.FS` with precompressed `.gz`/`.br` variants generated at build (`Content-Encoding` negotiation) — `ponytail: gzip only via a 30-line handler; brotli would need a dep`.
- GOMAXPROCS respects container CPU quota (Go 1.25 does this natively; otherwise `automaxprocs` — decided: rely on Go ≥ 1.25).
- `GOMEMLIMIT` set from `SHORTR_MEM_LIMIT` if provided to keep GC bounded in small containers.

### 19.4 Backpressure
- Click channel full → drop + metric (never block).
- DB slow → request contexts time out at 10 s → 503; `readyz` fails → proxy/orchestrator stops sending traffic; cached redirects **still work** during DB outage (redirects don't need DB on cache hit) — a key robustness property.
- `http.Server` `MaxHeaderBytes 64 KB`; body limits (§16); `MaxConns` not limited (Go handles 10k+ conns); OS `ulimit -n` documented (systemd `LimitNOFILE=65536`).

---

## 20. Reliability, Failure Modes & Recovery

| Failure | Behaviour | Recovery |
|---------|-----------|----------|
| Process crash | Committed clicks are safe (WAL / pg). In-flight batch (≤ 500 events / 250 ms) lost. Spool files replayed on start. | supervisor restarts (Docker `restart: unless-stopped`, systemd `Restart=always`) |
| SIGTERM | Stop accepting → wait in-flight ≤ `SHUTDOWN_TIMEOUT` → drain click queue → flush → close DB → exit 0 | |
| SQLite disk full | Writes fail → 503 on mutations; redirects continue from cache; clicks spool to disk fails too → dropped with metric; loud error log | free space; app self-heals (no restart needed) |
| SQLite corruption | Startup `PRAGMA quick_check` → refuse to start with guidance; restore from backups dir | `shortr restore --from backups/…` |
| Postgres down at start | Retry connect with backoff for `SHORTR_DB_CONNECT_TIMEOUT=60s` then exit 1 | orchestrator restarts |
| Postgres down at runtime | Mutations 503; redirects from cache OK; clicks spooled; `readyz` 503 | auto-reconnect via pool; spool replay |
| Migration fails midway | Each migration in a transaction (both DBs support DDL transactions) → rolled back; app exits 3 with message | fix & restart |
| Two instances migrate simultaneously (pg) | advisory lock serialises | |
| Two instances on one SQLite file | Unsupported; startup takes a lock file `$DATA_DIR/shortr.lock` (flock) → second instance exits with error | |
| OIDC provider down | Login via SSO fails with clear page; local login unaffected; existing sessions unaffected (no token refresh dependency) | |
| OIDC JWKS rotation | go-oidc refetches on unknown `kid` | |
| Clock skew (host) | JWT validation leeway 60 s; expiry checks on links use server time — documented | NTP |
| Cache/DB inconsistency | Write-through invalidation + 5 min TTL bound | |
| Click queue overflow | Drop + metric + warn log (sampled 1/min) | raise `SHORTR_CLICK_QUEUE` or scale |
| Memory pressure | cache capacity bound; rate-limit map bound; `GOMEMLIMIT` | |
| Bad `TRUSTED_PROXIES` | All clients appear as the proxy IP → rate limiting hits everyone. Startup warns if `BASE_URL` is https but no trusted proxies configured and `X-Forwarded-For` seen on first requests (log hint once). Admin System page shows "Detected proxy IP: 172.18.0.2 — is it in TRUSTED_PROXIES?" | |
| Upgrade with new migrations | Migrations run at start; backup taken first for SQLite automatically (`pre-migrate-<version>.db`) | |
| Downgrade | Not supported across migrations; documented | restore backup |

Health semantics: `healthz` = process alive; `readyz` = can serve mutations. Proxies should route on `healthz` (so cached redirects keep flowing during DB blips) — documented in NPM section.

---

## 21. Observability

- **Logs:** `slog` JSON to stdout. One line per request (method, path template — not raw code to avoid PII, status, duration, bytes, ip (per IP_MODE), request_id, user_id if any). Redirect logs at `debug` by default (`SHORTR_LOG_REDIRECTS=true` to raise to info) — at 10k/s logging every redirect is the bottleneck.
- **Request ID:** `X-Request-Id` honoured from trusted proxy else generated (ULID); echoed in response and error envelope.
- **Metrics** (`/metrics`, Prometheus text):
  `shortr_http_requests_total{route,method,status}`, `shortr_http_request_duration_seconds` (fixed buckets, histogram implemented by hand), `shortr_redirects_total{result=ok|notfound|gone|disabled|password}`, `shortr_cache_hits_total`, `shortr_cache_misses_total`, `shortr_cache_entries`, `shortr_clicks_queued`, `shortr_clicks_written_total`, `shortr_clicks_dropped_total`, `shortr_clicks_spooled_total`, `shortr_db_errors_total`, `shortr_ratelimited_total{scope}`, `shortr_links_total`, `shortr_users_total`, `shortr_build_info{version,commit}`, plus Go runtime (`go_goroutines`, `go_memstats_*` from `runtime/metrics`).
- **Admin System page** reads the same counters via `/api/v1/admin/system`.
- **pprof:** `SHORTR_PPROF=true` mounts `/debug/pprof` on a separate localhost-only listener `127.0.0.1:6060`.

---

## 22. Deployment

### 22.1 Build
```
make web      # cd web && npm ci && npm run build  → web/dist
make build    # CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(git describe)" -o bin/shortr ./cmd/shortr
make release  # cross-compile matrix: linux/amd64, linux/arm64, linux/arm/v7, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64; produces tar.gz/zip + sha256sums
```
Binary ≈ 25–30 MB (SQLite in pure Go + embedded SPA). UPX optional.

### 22.2 Dockerfile (multistage)
```
FROM node:22-alpine AS web        → build SPA
FROM golang:1.25-alpine AS build  → CGO_ENABLED=0 go build (copy web/dist in)
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shortr /shortr
VOLUME /data
EXPOSE 8080
USER nonroot
ENV SHORTR_DATA_DIR=/data
HEALTHCHECK --interval=30s CMD ["/shortr","healthcheck"]   # binary subcommand does the HTTP GET (no curl in distroless)
ENTRYPOINT ["/shortr"]
```
Multi-arch image via `docker buildx` (amd64, arm64, arm/v7). Image ≈ 30 MB.

### 22.3 docker-compose (SQLite default)
```yaml
services:
  shortr:
    image: ghcr.io/<org>/shortr:latest
    restart: unless-stopped
    environment:
      SHORTR_BASE_URL: https://sho.rt
      SHORTR_TRUSTED_PROXIES: 172.16.0.0/12
      SHORTR_SECRET_KEY: ${SHORTR_SECRET_KEY}
    volumes: [ "./data:/data" ]
    ports: [ "127.0.0.1:8080:8080" ]   # or none when on the NPM network
```
`docker-compose.postgres.yml` adds a `postgres:16-alpine` service with healthcheck and `depends_on: condition: service_healthy`.

### 22.4 systemd
`shortr.service`: `DynamicUser=yes`, `StateDirectory=shortr`, `EnvironmentFile=/etc/shortr/env`, `Restart=always`, `LimitNOFILE=65536`, `ProtectSystem=strict`, `NoNewPrivileges=yes`, `AmbientCapabilities=CAP_NET_BIND_SERVICE` (if binding :80/:443 directly).

### 22.5 Windows / macOS
Same binary; `shortr` runs as a console app; Windows service wrapper is out of scope (use NSSM/Task Scheduler — documented).

### 22.6 CLI subcommands
`shortr` (serve) · `shortr migrate [--to N]` · `shortr admin create|reset-password|promote` · `shortr backup [--out]` · `shortr restore --from` · `shortr config check` · `shortr healthcheck` · `shortr version`.

---

## 23. Nginx Proxy Manager Integration

### 23.1 Setup steps (documented in README)
1. Run NPM and shortr on the same Docker network (`docker-compose.npm.yml` defines `proxy` network).
2. NPM → Proxy Hosts → Add: Domain `sho.rt`, Scheme `http`, Forward host `shortr`, port `8080`. Enable **Block Common Exploits**, **Websockets Support** (harmless, not needed). SSL tab: request Let's Encrypt cert, **Force SSL**, **HTTP/2**, **HSTS**.
3. `SHORTR_BASE_URL=https://sho.rt`, `SHORTR_TRUSTED_PROXIES=<NPM container subnet, e.g. 172.18.0.0/16>`. NPM sets `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Real-IP` by default.
4. Optional "Advanced" tab custom config (`deploy/nginx-advanced.conf`):
```nginx
# Larger header buffers not needed; keep defaults. Pass request id through:
proxy_set_header X-Request-Id $request_id;
# Do not buffer streaming CSV exports
proxy_buffering off;
# Optional: cache-friendly short path timeouts
proxy_read_timeout 60s;
# Health for NPM's own checks (NPM has none; use uptime-kuma → /healthz)
```
5. If NPM is on another host, set `TRUSTED_PROXIES` to that host's IP.

### 23.2 Real IP algorithm
```
remote := ip(RemoteAddr)
if remote ∉ TRUSTED_PROXIES → client = remote (ignore headers)
else:
  chain := split(REAL_IP_HEADER)          // X-Forwarded-For: client, proxy1, proxy2
  walk from right to left, skipping trusted IPs; first untrusted = client
  if all trusted or empty → client = leftmost (or remote if header empty)
validate parse; on garbage → client = remote
```
Also derive `scheme` from `X-Forwarded-Proto` (trusted only) for correct cookie `Secure` and absolute URLs; `X-Forwarded-Host` is **not** trusted for building short URLs — `BASE_URL` is authoritative (prevents host-header injection).

### 23.3 Other proxies
Same settings work for Traefik, Caddy, Cloudflare Tunnel (`REAL_IP_HEADER=CF-Connecting-IP`, trusted = Cloudflare ranges or the `cloudflared` container IP).

---

## 24. Edge Cases Catalogue

Grouped; each with handling.

### 24.1 Links & codes
| Case | Handling |
|------|----------|
| Alias collides with existing (any case) | 409 `CODE_TAKEN`; UI live check |
| Alias collides with soft-deleted link | 409 during 30-day reservation; message says "recently deleted, available on <date>"; admin may purge to free |
| Generated code collides (astronomically rare) | retry ×5, then +1 length |
| Alias is a reserved route | 409 with reason |
| Code contains chars outside alphabet in the request path (`/abc%20`, `/abc.`) | 404 immediately, no DB |
| Trailing slash `/abc/` | `ServeMux` treats differently; explicit redirect `/abc/` → `/abc` (301) |
| Uppercase variant `/ABC` of `abc` | case-insensitive fallback resolves; counted as the same link |
| Very long URL (> limit) | 400 with limit stated |
| URL with spaces / unencoded unicode | reject with hint to encode; UI offers "Encode automatically" button that applies `encodeURI` client-side |
| Target is the shortener itself / another short link of ours | reject self-domain (loop) unless admin; other shorteners allowed |
| Target redirects elsewhere | we don't follow; title fetch follows ≤ 3 for the title only |
| Same user shortens same URL twice | new link (explicit design: analytics per campaign). UI shows "You already have 2 links to this URL" hint (dedupe query on create, non-blocking) |
| Duplicate URL detection | `?dedupe=true` API param returns existing link instead of creating |
| Fragment `#section` in target | preserved verbatim |
| Query merge produces duplicate keys | short-URL params appended after target's; server doesn't dedupe (target site decides) |
| Query merge > 2 KB | append skipped, redirect to raw target, event flagged `qs_dropped` |
| `pass_query=false` but link has utm | utm still appended |
| Expiry time in the past on create | 400 |
| Expiry passes while cached | cache entry stores `expires_at`; checked at request time — no stale serving |
| Click-limit boundary under concurrency | atomic in-memory counter decides; may allow a few extra hits under bursts across restarts (DB count lags ≤ 250 ms); documented |
| Restart resets in-memory counters | seeded from DB `click_count` on cache load; batch writer lag ≤ 250 ms → at most a handful over |
| Disabled vs deleted vs expired | 404 "disabled" page / 410 gone / 410 expired; all noindex; API distinguishes via `status` |
| Restore after 30 d | not possible (purged); UI hides restore |
| Change code of existing link | allowed; old code becomes free immediately (no reservation — it was the owner's choice); warning in UI that old short URL breaks |
| Owner deleted | link `user_id=NULL` (if "keep"), still redirects; admin can reassign |
| Password link + HEAD | HEAD returns 200 (form) — link checkers see it as OK |
| Password link + bots | bots get the form; never see target |
| Password cookie theft | cookie is HMAC(link id, pw hash version, expiry 1 h); changing the password invalidates; scoped `Path=/` but name-bound to code |
| Password brute force | 5/min/IP/link + 429 page; argon2 cost also slows |
| Link with 301 changed later | browsers cache 301 permanently — UI warns when choosing 301 and when editing target of a 301 link; default is 302 |
| 307/308 with POST | method preserved; documented for API use cases |
| Client sends `If-None-Match` etc. on redirect | ignored; always fresh |
| Unicode alias | rejected (ASCII only) to keep URLs unambiguous |
| Alias `api` / `app` | reserved |
| Case: `/App` | not reserved-matched (routes are exact), resolves as a code → 404 unless exists; fine |
| Robots | `/robots.txt` → `Disallow: /` except `/app` allowed? Everything disallowed; short links use `X-Robots-Tag` |
| Favicon requests on short domain | `/favicon.ico` served from embed (small), not counted |

### 24.2 Analytics
| Case | Handling |
|------|----------|
| Link preview bots inflate counts | classified bot, excluded by default |
| Missing UA | `device=other`, flagged bot |
| Spoofed UA | accepted; nothing to do |
| IPv6 | stored; anonymised /48; rate-limit /64 |
| IPv4-mapped IPv6 (`::ffff:1.2.3.4`) | unmapped before use |
| Multiple XFF entries / garbage | algorithm §23.2; garbage → RemoteAddr |
| Private IP as client (misconfig) | logged hint once; GeoIP empty |
| GeoIP file missing/corrupt | startup warning, feature off; hot-reload on SIGHUP or admin "reload GeoIP" |
| GeoIP file updated on disk | reopened by daily job if mtime changed |
| Referrer is the short domain (password page → redirect) | ignored → `direct` |
| Referrer with credentials/long | truncated 512, userinfo stripped |
| Same visitor many clicks | all counted as clicks; uniques by IP/day |
| Clock changes / DST | all storage UTC; bucketing by tz done at query time; DST days show 23/25-hour buckets — inherent |
| Timezone unknown/invalid | 400; UI defaults to browser tz |
| Range too large | 400 `span ≤ 2y`; UI limits picker |
| Rollup runs while retention deletes | rollup first, deletion only for days with `rolled=true` watermark |
| Retention 0 | never delete |
| Export of 10M rows | streamed; 60 s write deadline extended per chunk; client can also paginate `/clicks` |
| CSV injection | escaped |
| Click for a link deleted moments ago | inserted anyway (no FK); purge job deletes clicks of purged links |
| Queue overflow | drop + metric |
| Counting HEAD / OPTIONS | never |
| `click_count` drift vs raw rows | nightly reconciliation job recomputes counts from rollups+raw for links touched that day (cheap) |

### 24.3 Auth / users
| Case | Handling |
|------|----------|
| Setup race | transaction + count check → second gets 409 |
| Last admin demote/disable/delete | 409 `LAST_ADMIN` |
| Admin disables self | allowed only if another admin exists |
| Email case variants | lowercased; unique |
| Change email to one used by another | 409 |
| OIDC-only user tries local login | generic "invalid credentials" (no info leak); page hints "Use SSO if your account was created via SSO" generically |
| User disabled mid-session | next request 403 `USER_DISABLED`, session deleted |
| Password change on device A | device B logged out (`password_changed_at`) |
| Session cookie from old SECRET_KEY | sessions are DB rows, not signed → survive key rotation; OIDC/CSRF cookies don't (fine, short-lived) |
| SECRET_KEY lost | app regenerates; only in-flight OIDC/password-link cookies invalid |
| API key used after user disabled | 403 |
| API key with insufficient scope | 403 `FORBIDDEN` naming the scope |
| API key mints key | 403 (session only) |
| Lockout vs legit user | 15 min; admin can clear via user page (in-memory clear) |
| Registration open + OIDC auto-create | both create `user` role |
| Invite mode | admin creates user with one-time password; `must_change_password=true` forces change at first login (`ponytail: flag column, no invite tokens/email`) |
| Concurrent PATCH of a link | ETag 412 |
| `next` param abuse | validated |
| Cookie too large / blocked | OIDC cookie ~600 B; if browser blocks cookies → OIDC callback error `OIDC_STATE_MISSING` with hint |
| Third-party cookie contexts (extension iframes) | extension uses API keys, not cookies |

### 24.4 OIDC
| Case | Handling |
|------|----------|
| `state` mismatch / missing cookie | error page, log; user retries |
| `nonce` mismatch | reject token |
| Provider returns `error=access_denied` | friendly page "Sign-in cancelled" |
| Token without `email` | if identity exists → fine; else try userinfo; else `OIDC_EMAIL_UNVERIFIED`/`OIDC_NO_ACCOUNT` |
| Email changed at IdP | identity's stored email updated; user's `email` **not** auto-changed (could collide) — admin notice on user page "IdP email differs" |
| Same email, different IdP subject (user recreated at IdP) | not auto-linked (subject differs) unless `AUTO_LINK_BY_EMAIL`; then link second identity — flagged in audit |
| Linking an identity already linked to another user | 409 `OIDC_ALREADY_LINKED` |
| Unlink last method | 409 `CANNOT_UNLINK` |
| Issuer URL changed (migration to new IdP) | identities keyed by old issuer become dead; admin CLI `shortr oidc reissuer --from --to` rewrites |
| Discovery fails at startup | if `OIDC_ENABLED`: start anyway, mark OIDC degraded (button shows "temporarily unavailable"), retry discovery every 60 s; `readyz` still OK (SSO isn't core). `SHORTR_OIDC_REQUIRED=true` makes it fatal |
| Group claim as string not array | both accepted |
| ID token huge (Azure groups overage) | claims size cap 32 KB; overage → groups ignored + warn |
| Clock skew | 60 s leeway |
| Admin group removed | demoted on next login unless `role_locked` |

### 24.5 Ops
| Case | Handling |
|------|----------|
| Port in use | exit 1 with message |
| DATA_DIR not writable | exit 2 |
| SQLite on NFS/SMB | warn at start (locking unreliable); documented |
| Docker volume perms (non-root UID 65532) | README: `chown -R 65532:65532 ./data`; startup error message says exactly this |
| Big SQLite (> 5 GB clicks) | works; recommend Postgres or retention; VACUUM job optional weekly |
| Backup during heavy writes | `VACUUM INTO` is consistent |
| `BASE_URL` changed | short URLs displayed change; nothing stored depends on it |
| Trusted proxies wrong | admin System page hint |
| Metrics scraped from internet | token or proxy ACL required; default listens on same port but 403 without token when `METRICS_TOKEN` set; if unset, only allowed from `TRUSTED_PROXIES`/loopback |
| Time goes backwards (VM restore) | ULIDs remain unique (random component); expiry might misbehave briefly |
| Very many users/links | list endpoints paginated; admin counts via cached counters refreshed per minute |

---

## 25. Limitations

1. **Single-node writes.** SQLite = one process. Postgres mode allows multiple replicas in principle, but caches/rate limits/idempotency/lockouts are per-node, so limits are approximate ×N and cache staleness up to 5 min. Not a supported topology in v1.
2. **One OIDC provider.**
3. **No email delivery** (password reset by admin; invites by one-time password).
4. **Uniques are approximate** (IP-based, worse under `anonymize`).
5. **GeoIP requires a user-supplied database** (MaxMind licence) — no bundled data.
6. **Bot detection is UA-based**; sophisticated bots are counted.
7. **Timezone bucketing** for hour granularity uses ts shift; for day+ uses UTC days shifted in UI (edge-of-day clicks can move a day). Fix path: bucket in SQL with tz offset per query (Postgres `AT TIME ZONE`; SQLite via offset seconds).
8. **Click-limit precision** ±few under concurrency/restart.
9. **No custom domains per user**; one base URL.
10. **No ACME**; TLS via proxy or provided certs.
11. **Browser-cached 301s** can't be revoked.
12. **In-memory lockout/idempotency** reset on restart.
13. **Windows service** wrapper not provided.
14. **No 2FA/TOTP** in v1 (schema-ready; planned v1.2).
15. **Title fetch** is an outbound request (can be disabled entirely).

---

## 26. Future Clients: Android App & Browser Extension

Design decisions already made to support them:

- **API keys** with scopes; created in web UI, pasted into client once. Later: device authorization / OIDC PKCE in-app (public client, `redirect_uri` `shortr://callback`) issuing an API key via `POST /api/v1/apikeys/exchange` — schema and flow reserved.
- **CORS** allowlist for `chrome-extension://` / `moz-extension://` origins (admin setting); extensions use `Authorization` header, no cookies → no CSRF concerns.
- **Bulk & preview endpoints** for "shorten all tabs".
- **`GET /api/v1/links/check`** for live alias validation.
- **QR PNG endpoint** for sharing from mobile.
- **Android Share Target:** app receives text → `POST /links` → copies short URL → notification with QR/copy actions. Uses `/api/v1/me` to display quota/user.
- **Extension:** popup = mini create form + last 5 links; context menu "Shorten this link"; options page for server URL + API key; badge with click count via `/links/{id}` polling.
- **Stable API versioning**: `/api/v1` frozen after 1.0; breaking changes → `/api/v2`. `Deprecation` header support planned.
- **Offline-friendly**: create is idempotent via `Idempotency-Key` so retries from flaky mobile networks are safe.
- **Rate limits** are per API key so one misbehaving device doesn't block the user's web session.

---

## 27. Testing Strategy

| Layer | Approach |
|-------|----------|
| Unit | code generation (alphabet, no bias via chi-square-ish sanity), validation table tests, real-IP algorithm, UA classification, cache LRU/TTL/negative, rate limiter, query merge, `next` validation, argon2 round trip |
| Store | run the same test suite against SQLite (in-memory) and Postgres (skipped unless `TEST_PG_DSN`) — dialect parity |
| HTTP | `httptest` end-to-end: setup → login → create → redirect → click recorded (flush) → stats; CSRF rejection; API key scopes; ETag 412; rate limit 429; password link flow; expiry/410 |
| OIDC | fake IdP in-process (`httptest.Server` serving discovery, JWKS, token endpoint) covering all resolution branches in §12.1 |
| Migration | apply all from 0 on both DBs; `PRAGMA integrity_check` |
| Concurrency | `go test -race`; click writer under 100 goroutines; click-limit boundary |
| Load | `hey -z 30s -c 100 http://localhost:8080/abc` in CI (soft threshold), Go benchmarks for cache & redirect handler |
| Frontend | Vitest for schemas/api client; Playwright smoke: setup → create → open short URL → see click in UI; axe accessibility check on main pages |
| Security | `govulncheck`, `gosec` (advisory), `npm audit`; manual checklist §18 |
| Fuzz | `go test -fuzz` on URL validator and code parser |

CI (GitHub Actions): lint → unit → store(pg service) → build web → build binary → e2e → docker build multi-arch (on tag) → release.

---

## 28. Implementation Phases

| Phase | Deliverable | Done when |
|-------|-------------|-----------|
| **0. Scaffold** | go.mod, config, slog, server skeleton, healthz, embed placeholder, Dockerfile, Makefile | `docker run` serves `/healthz` |
| **1. Store & schema** | migrations (both dialects), Store, ULID, store tests | tests pass on sqlite+pg |
| **2. Core links** | code gen, validation, create/get/list/patch/delete, cache, redirect handler, public pages | curl create → redirect works |
| **3. Clicks & analytics** | event pipeline, UA/GeoIP/IP modes, rollups, retention, stats queries, export | stats endpoints return correct numbers in tests |
| **4. Auth** | setup, local login, sessions, CSRF, API keys, roles, admin user mgmt, audit | protected API works with cookie & key |
| **5. OIDC** | full flow, auto-create/link toggles, linking UI endpoints, fake-IdP tests | all §12 branches tested |
| **6. Frontend** | SPA per §14–15, embedded; a11y pass; mobile pass | Playwright smoke green |
| **7. Ops polish** | metrics, readyz, backups, spool/replay, graceful shutdown, rate limits, NPM docs, compose files, systemd | load test meets §19.1 |
| **8. Release 1.0** | README, API docs, CHANGELOG, multi-arch images, release binaries | tag v1.0.0 |
| Later | 1.1 SMTP (reset/invite emails) · 1.2 TOTP 2FA · 1.3 custom domains · 1.4 multi-IdP · Android app · extension |

Each phase ships runnable; nothing is scaffolded ahead of its phase.

---

## 29. Open Decisions (defaults chosen)

| Decision | Default | Rationale / change if |
|----------|---------|-----------------------|
| Default redirect status | 302 | editable links + accurate analytics; 301 opt-in per link |
| Default alphabet | base58 | human readability; base62 if you want shorter codes |
| Default IP mode | anonymize | privacy-safe default; `full` if you need exact IPs |
| Count bots | off | matches user expectation |
| Registration | closed | admin creates or OIDC auto-create |
| OIDC auto-create / auto-link | off / off | secure default; flip in admin settings |
| Title fetch | on | convenience; off for zero-egress deployments |
| Soft-delete reservation | 30 d | anti-phishing reuse |
| Session TTL | 30 d sliding, 90 d absolute | |
| Config file format | JSON (env preferred) | no YAML dep |
| Project name | `shortr` | placeholder; rename is a find-replace |

---

## 30. Glossary

- **Code / alias** — the path segment after the base URL (`sho.rt/<code>`). Alias = user-chosen code.
- **Click** — one counted GET on a short link (non-bot by default).
- **Rollup** — pre-aggregated daily stats row.
- **Sudo mode** — a window (10 min) after re-authentication required for sensitive actions.
- **Trusted proxy** — an IP/CIDR whose forwarding headers we believe.
- **Identity** — an `(issuer, subject)` pair from an OIDC provider linked to a user.
- **Spool** — on-disk JSONL fallback for clicks when the DB is unavailable.
