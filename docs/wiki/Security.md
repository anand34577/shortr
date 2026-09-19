# Security

This page summarizes the security model so operators know what's handled
for them and what's their responsibility. Shortr has been through a full
audit pass (auth, database, business logic, HTTP surface, deployment
config) — see the git history for details of what was found and fixed.

## Handled for you

- **Passwords:** argon2id (t=3, 64MB memory), constant-time comparison,
  automatic rehash-on-upgrade if parameters change, and a dummy-hash timing
  defense so login timing doesn't reveal whether an email exists.
- **Sessions & API keys:** opaque random tokens; only a SHA-256 hash is ever
  stored, so a database leak alone doesn't hand out live credentials.
  Sessions are invalidated on password change or account disable.
- **CSRF:** double-submit token + `Origin` check on every session-cookie
  mutating request. Bearer/API-key requests are exempt (they carry no
  ambient cookie, so CSRF doesn't apply to them).
- **CORS:** explicit allowlist via `SHORTR_CORS_ORIGINS`; never a wildcard
  combined with credentials.
- **SSRF:** link-target and title-fetch validation blocks
  loopback/private/link-local/CGNAT ranges by default
  (`SHORTR_ALLOW_PRIVATE_TARGETS=false`), and the title-fetcher re-checks
  the *actual connected IP* at dial time — not just the hostname — which
  defeats DNS-rebinding attacks. Redirects during title fetch are capped at
  3 hops, each re-validated.
- **API key scopes:** `links:read`, `links:write`, `stats:read`, `admin:*`.
  A key minted with a narrow scope cannot escalate to admin functionality
  even if it belongs to an admin user — every admin route checks both
  `user.IsAdmin()` and, for API-key callers, the `admin:*` scope.
- **Open redirect:** post-login/OIDC redirect targets are validated to start
  with `/app` and rejected if they contain `//` or a backslash.
- **Rate limiting:** separate limiters for login/register/OIDC-start (keyed
  by email *and* IP), general API traffic (keyed by user/API-key, falling
  back to IP), and the redirect hot path — plus a self-limit on the sudo
  (step-up re-auth) endpoint.
- **Panic isolation:** a top-level recover middleware means a single bad
  request can't crash the process; the async click-writer additionally
  recovers per-event so a malformed GeoIP record or user-agent string drops
  one click, not the entire analytics pipeline.
- **Secrets:** never logged in plaintext (`Config.Redacted()` masks them
  before the startup config log line), and the auto-generated
  `secret.key` file is written with `0600` permissions.

## Your responsibility as an operator

- **Set `SHORTR_TRUSTED_PROXIES`** if you're behind any reverse proxy —
  otherwise real client IPs (and therefore analytics + rate limiting)
  silently degrade to "everything is the proxy's IP."
- **Set `SHORTR_SECRET_KEY` explicitly** for multi-instance or
  redeploy-to-a-different-host setups; the auto-generated one is tied to the
  data directory it's stored in.
- **Don't enable `SHORTR_ALLOW_PRIVATE_TARGETS`** unless you specifically
  need internal-network short links — it's a global, all-users toggle.
- **Scope API keys narrowly.** Especially for MCP/agent use — give an agent
  `links:write` only if that's all it needs, not `admin:*`.
- **Keep GeoIP/SMTP/Gotify credentials out of version control** — they're
  environment variables for a reason; `.env` is gitignored by default.
- **Rotate a leaked API key or session** immediately: Settings → API Keys
  (revoke) or Settings → Security → Sessions (revoke), or as an admin,
  `DELETE /api/v1/users/{id}/sessions`.

## Reporting a vulnerability

If you find a security issue, please **don't** open a public GitHub issue.
Instead email the maintainer directly (see the repository's contact info)
with reproduction steps; a fix and coordinated disclosure will follow.
