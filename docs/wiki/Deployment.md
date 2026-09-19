# Deployment

## Reverse proxy / TLS

Shortr's own HTTP server speaks plain HTTP only — terminate TLS in front of
it (Nginx Proxy Manager, Traefik, Caddy, Cloudflare Tunnel, or a bare nginx).
Whatever you use, make sure it:

1. Forwards the real client IP (`X-Forwarded-For` by default), and
2. Its own address (or the whole proxy network) is listed in
   `SHORTR_TRUSTED_PROXIES`.

Skipping step 2 means Shortr ignores the forwarded header entirely — every
click and every rate-limit bucket gets attributed to the proxy's IP, which
silently breaks analytics and can let one abusive client exhaust the rate
limit for everyone. See
[deploy/NGINX_PROXY_MANAGER.md](../../deploy/NGINX_PROXY_MANAGER.md) for a
worked example, and `deploy/nginx-advanced.conf` for a hand-rolled nginx
config.

## Docker Compose variants

| File | Use case |
|---|---|
| `docker-compose.yml` | SQLite, no proxy — good for a quick trial or a single-user instance behind your own reverse proxy on the host. |
| `deploy/docker-compose.postgres.yml` | PostgreSQL backend, for multi-GB click volumes or when you already run Postgres. |
| `deploy/docker-compose.npm.yml` | Wired for [Nginx Proxy Manager](https://nginxproxymanager.com/) — Shortr isn't published on a host port, only reachable via the NPM network. |

All variants read secrets from environment variables with no committed
defaults; `SHORTR_SECRET_KEY` and `POSTGRES_PASSWORD` (postgres variant) use
`${VAR:?required}` syntax so `docker compose up` fails loudly instead of
starting with an empty secret.

## systemd

`deploy/shortr.service` runs the binary directly with:

- `DynamicUser=yes` — no shared system account
- `NoNewPrivileges=yes`, `ProtectSystem=strict`, `ProtectHome=yes`,
  `PrivateTmp=yes`
- `Restart=always`, `RestartSec=2`
- A `StateDirectory=` scoped `ReadWritePaths`

See [Installation](Installation.md#option-3--systemd-bare-metal--vm) for the
install steps.

## Horizontal scaling

Shortr is **not** designed for multiple concurrently-running replicas behind
a load balancer in v1: rate-limit buckets, the redirect-path in-memory link
cache, and idempotency keys are all per-process. Running two instances
against the same database will work functionally, but each instance
enforces its own rate limits and can serve a stale cached link briefly after
another instance updates it. If you need more redirect throughput than one
instance provides, scale vertically first; see
[PLAN.md §25](../../PLAN.md#25-limitations) for the full list of v1
limitations.

## Observability

- `GET /healthz` — liveness (process is up).
- `GET /readyz` — readiness (database is reachable).
- `GET /metrics` — Prometheus text exposition; gate it with
  `SHORTR_METRICS_TOKEN` if it's reachable from outside your monitoring
  network.
- Structured JSON logs to stdout by default (`SHORTR_LOG_FORMAT=text` for
  human-readable); `SHORTR_LOG_LEVEL` controls verbosity.

## Updating

```bash
docker compose pull && docker compose up -d
# or, bare metal:
sudo systemctl stop shortr
sudo cp shortr-new /opt/shortr/shortr
sudo systemctl start shortr
```

Database migrations run automatically at startup (`shortr migrate` also
exists to apply them without starting the server, e.g. in a CI/CD step
before a rolling restart). Migrations are tracked in `schema_migrations` and
safe to run repeatedly — an already-applied migration is skipped.
