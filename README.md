# Shortr

[![CI](https://github.com/anand34577/shortr/actions/workflows/ci.yml/badge.svg)](https://github.com/anand34577/shortr/actions/workflows/ci.yml)
[![Release](https://github.com/anand34577/shortr/actions/workflows/release.yml/badge.svg)](https://github.com/anand34577/shortr/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A self-hosted URL shortener: single static Go binary (or a ~30 MB Docker
image), an embedded React admin UI, no required third-party services, and
enough analytics, auth, and notification plumbing to run for one person or
a whole team.

**[Wiki](docs/wiki/Home.md)** — installation, full configuration
reference, API/MCP reference, deployment, backup/restore, security, and
troubleshooting.

Full design document: [PLAN.md](PLAN.md) — architecture, data model, every
API endpoint, validation rule, error code, and ~110 documented edge cases.

## Highlights

- **Single binary / single container.** SQLite by default (pure-Go driver,
  no CGO), PostgreSQL if you want it — same binary, same feature set.
- **No required third-party dependency.** OIDC, SMTP, and Gotify push are
  all optional and self-configured; nothing calls out to any hosted service
  unless you point it at your own.
- **Real analytics.** IP (with a privacy mode: full/anonymized/hashed/off),
  country/region/city via an optional offline MaxMind database, device/OS/
  browser, referrers, UTM, bot filtering (including chat-app link
  unfurlers), CSV export.
- **Auth for one or many users.** Local email+password, OIDC/SSO with an
  auto-create toggle and account linking, per-user API keys for the planned
  Android app and browser extension, admin user management, audit log.
- **Notifications.** In-app (with browser `Notification` API support in the
  UI) always; optional email (SMTP) and optional self-hosted Gotify push,
  per-notification-kind opt-in/out.
- **Reverse-proxy ready.** Correct client-IP resolution behind Nginx Proxy
  Manager, Traefik, Caddy, or Cloudflare Tunnel — see
  [deploy/NGINX_PROXY_MANAGER.md](deploy/NGINX_PROXY_MANAGER.md).
- **API-first.** A full REST API with scoped, per-user API keys
  (`links:read`/`links:write`/`stats:read`/`admin:*`), a live OpenAPI 3 spec
  at `/api/v1/openapi.json`, and an [MCP server](docs/wiki/MCP-Server.md) so
  AI agents can manage links using the same scoped keys.

## Quick start (Docker)

```bash
cp .env.example .env      # set SHORTR_BASE_URL at minimum
docker compose up -d
```

Then open `http://localhost:8080/app/setup` once to create the first admin
account. PostgreSQL instead of SQLite: `docker compose -f
deploy/docker-compose.postgres.yml up -d`. Behind Nginx Proxy Manager:
`docker compose -f deploy/docker-compose.npm.yml up -d` and see
[deploy/NGINX_PROXY_MANAGER.md](deploy/NGINX_PROXY_MANAGER.md).

## Quick start (binary)

Requires Go 1.25+ and Node 20+ to build from source.

```bash
make build          # builds web/ then the Go binary into bin/shortr
SHORTR_BASE_URL=http://localhost:8080 ./bin/shortr
```

Or grab a prebuilt binary for your platform from the
[Releases page](../../releases) — CI cross-compiles
linux/darwin/windows × amd64/arm64 and publishes a `SHA256SUMS` file on
every tagged push (`make release` does the same thing locally).

Every setting is an environment variable prefixed `SHORTR_`; see the
[Configuration wiki page](docs/wiki/Configuration.md) or
[PLAN.md §6](PLAN.md#6-configuration) for the full list, or run
`shortr config check` to validate and print the resolved configuration.

## CLI

```
shortr [serve]                 Start the server (default with no args)
shortr migrate                 Apply pending database migrations and exit
shortr admin create --email a@b.com [--password ...] [--name ...]
shortr admin reset-password --email a@b.com [--password ...]
shortr admin promote --email a@b.com
shortr backup [--out path.db]  On-demand SQLite backup (VACUUM INTO)
shortr config check            Validate configuration and exit
shortr healthcheck             Exit 0 if the local server is healthy
shortr version
```

## Development

```bash
# backend
go test ./...
go run ./cmd/shortr

# frontend (proxies /api, /auth to a backend running on :8080 by default)
cd web && npm install && npm run dev
```

## Project layout

```
cmd/shortr/        entrypoint, CLI subcommands
internal/config/    env-var configuration, validated at startup
internal/store/      SQLite/PostgreSQL, migrations, all SQL
internal/link/        short-code generation, redirect cache, link CRUD
internal/click/        UA/GeoIP/IP-privacy enrichment, batched click writer
internal/auth/        password hashing, sessions, API keys, OIDC
internal/notify/       in-app + SMTP + Gotify notification fan-out
internal/ratelimit/    in-memory token-bucket rate limiting
internal/metrics/      hand-rolled Prometheus text exposition
internal/jobs/         background maintenance (retention, backups, GC)
internal/server/       HTTP API, redirect handler, public pages, SPA serving
web/                the React admin UI (Vite + TypeScript)
deploy/             Docker Compose variants, systemd unit, NPM guide
docs/wiki/          operator/integrator wiki (see docs/wiki/Home.md)
.github/workflows/  CI (build/vet/test) and tagged-release automation
```

## What's deliberately not built (v1)

See [PLAN.md §25](PLAN.md#25-limitations) for the full list. In short: one
OIDC provider at a time, no outbound email for password resets (admin resets
or SSO instead), analytics uniques are IP-based and approximate under the
default anonymized IP mode, and horizontal scaling to multiple replicas
isn't a supported topology (rate limits, caches, and idempotency keys are
per-node).

## License

MIT — see [LICENSE](LICENSE).
