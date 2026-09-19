# Contributing

## Prerequisites

- Go 1.26+
- Node 20+
- (optional) a MaxMind GeoLite2 `.mmdb` file if you're touching GeoIP code

## Running locally

```bash
# backend
go run ./cmd/shortr

# frontend, in a second terminal — proxies /api and /auth to :8080
cd web && npm install && npm run dev
```

## Tests

```bash
go test ./...            # full backend suite
go test ./... -race      # with the race detector (CI runs this)
go vet ./...

cd web && npm run build  # tsc -b + vite build; catches type errors
cd web && npm run lint   # oxlint
```

CI (`.github/workflows/ci.yml`) runs the frontend build, `go vet`,
`go build`, `go test -race`, and a Docker build check on every push/PR to
`main`/`master`.

## Project layout

```
cmd/shortr/          entrypoint, CLI subcommands
internal/config/     env-var configuration, validated at startup
internal/store/      SQLite/PostgreSQL, migrations, all SQL
internal/link/       short-code generation, redirect cache, link CRUD
internal/click/      UA/GeoIP/IP-privacy enrichment, batched click writer
internal/auth/       password hashing, sessions, API keys, OIDC
internal/notify/     in-app + SMTP + Gotify notification fan-out
internal/ratelimit/  in-memory token-bucket rate limiting
internal/metrics/    hand-rolled Prometheus text exposition
internal/jobs/       background maintenance (retention, backups, GC)
internal/server/     HTTP API, redirect handler, public pages, SPA serving,
                      MCP server, OpenAPI spec generation
internal/iploc/      IP geolocation/ASN lookup for the admin IP-lookup tool
web/                 React admin UI (Vite + TypeScript)
deploy/              Docker Compose variants, systemd unit, NPM guide
docs/wiki/           this wiki
```

Full design rationale (data model, every API field, every error code, and
~110 documented edge cases) is in [PLAN.md](../../PLAN.md) — read it before
making a non-trivial change; a lot of "why is it done this way" is answered
there.

## Conventions

- No comments explaining *what* code does — only *why*, when it's genuinely
  non-obvious (a hidden constraint, a workaround, a subtle invariant).
- Prefer editing existing files over adding new abstractions; this codebase
  deliberately avoids speculative flexibility ("ponytail" comments in the
  code mark places where a simpler option was chosen on purpose — see
  inline notes for the reasoning).
- Every SQL query is parameterized — never string-concatenate user input
  into a query, even for column/dimension names (use a whitelist map, as the
  analytics breakdown queries do).
- New background work should recover from panics per-unit-of-work (see
  `internal/click/writer.go`'s `safeEnrich` or `internal/jobs/jobs.go`'s
  `runOnce`) rather than letting one bad event kill a long-running
  goroutine.

## Submitting changes

1. Fork, branch, make your change with tests.
2. `go test ./... -race` and `cd web && npm run build` must pass.
3. Open a PR describing *why*, not just *what* — link an issue if there is
   one.

## Releasing (maintainers)

Push a semver tag; `.github/workflows/release.yml` builds the frontend,
cross-compiles linux/darwin/windows × amd64/arm64, publishes a GitHub
Release with a `SHA256SUMS` file, and builds/pushes a multi-arch Docker
image to `ghcr.io/<org>/<repo>`.

```bash
git tag v1.2.3
git push origin v1.2.3
```
