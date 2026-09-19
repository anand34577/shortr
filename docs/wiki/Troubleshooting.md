# Troubleshooting

## Server won't start: `config error: ...`

Config is validated fail-fast — the message names the exact variable and
why it's invalid. Run `shortr config check` to see the full resolved config
without starting the server. Common ones:

- `SHORTR_BASE_URL: invalid URL` — must include a scheme (`http://` /
  `https://`), no trailing content beyond host[:port].
- `SHORTR_DB_DSN is required when SHORTR_DB_DRIVER=postgres` — set a full
  Postgres connection string.
- `SHORTR_SECRET_KEY must be at least 32 characters` — either remove it (let
  Shortr auto-generate one) or use a longer value, e.g.
  `openssl rand -hex 32`.
- `SHORTR_OIDC_ISSUER must be https` — either use an HTTPS issuer or set
  `SHORTR_OIDC_INSECURE_HTTP=true` for local testing only.

## `/readyz` returns non-200 but `/healthz` is fine

The process is up but the database isn't reachable. For SQLite, check
`SHORTR_DATA_DIR` is writable and not on a network filesystem that doesn't
support proper file locking. For Postgres, check `SHORTR_DB_DSN`,
network/firewall rules, and that the Postgres container/service is actually
healthy (`docker compose ps`).

## Analytics show every visitor as the same IP / country

`SHORTR_TRUSTED_PROXIES` isn't set (or doesn't include your proxy), so
Shortr is trusting the TCP peer address instead of `X-Forwarded-For` — which
is your reverse proxy's own IP for every request. Set it to your proxy's
address or the Docker network CIDR it lives on. See
[Deployment](Deployment.md#reverse-proxy--tls).

## Rate limited unexpectedly (`429 RATE_LIMITED`)

Check which limiter: the response includes a `Retry-After` header. If it's
happening for legitimate traffic behind a shared NAT/proxy IP, the same
`TRUSTED_PROXIES` misconfiguration above is often the cause — traffic that
should be attributed per-user/per-real-IP is instead bucketed under one IP.
Otherwise, raise the relevant `SHORTR_RATE_LIMIT_*` variable.

## Emails / Gotify pushes never arrive

- Check **Admin → Settings** (or `SHORTR_SMTP_ENABLED` /
  `SHORTR_GOTIFY_ENABLED`) is actually on.
- Notification delivery is best-effort and logged, not surfaced to the UI as
  an error — check server logs (`SHORTR_LOG_LEVEL=debug` for more detail) for
  `smtp dial:` / `smtp auth:` / Gotify HTTP errors.
- SMTP sends have a 10s timeout; a consistently slow/unreachable host will
  show timeout errors in the log rather than hanging the server.

## Clicks seem to be missing after a crash/restart

Click writes are batched and asynchronous by design (so the redirect path
never blocks on a database write). On a database outage, batches are
spooled to `<DATA_DIR>/clicks-spool/*.jsonl` and automatically replayed on
next startup — check the log for `replayed spooled clicks`. On a graceful
shutdown (SIGTERM), the writer is given up to `SHORTR_SHUTDOWN_TIMEOUT` to
flush before the process exits.

## GeoIP columns (country/city) are always empty

`SHORTR_GEOIP_DB` isn't set, or points to a file that failed to open — check
startup logs for `GeoIP database could not be opened`. Download a
GeoLite2-City `.mmdb` from MaxMind (free account required) and point
`SHORTR_GEOIP_DB` at it.

## MCP tool calls return "MCP server is disabled"

An admin needs to enable it first: **Admin → Settings → MCP Server**. See
[MCP Server](MCP-Server.md).

## Still stuck?

Open an issue with: `shortr version` output, relevant log lines (redact
secrets/tokens), and your `SHORTR_*` config with secrets stripped
(`shortr config check` already masks them).
