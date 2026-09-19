# Shortr Wiki

Shortr is a self-hosted URL shortener: a single static Go binary (or a small
Docker image) with an embedded React admin UI, SQLite or PostgreSQL storage,
real click analytics, local/OIDC auth, and optional email/push notifications.
No third-party SaaS dependency is ever required.

This wiki is the operator- and integrator-facing companion to the top-level
[README](../../README.md) (quick start) and [PLAN.md](../../PLAN.md) (the
full design document: data model, every API endpoint, validation rule, error
code, and edge case).

## Pages

- **[Installation](Installation.md)** — Docker, Docker Compose (SQLite /
  PostgreSQL / behind Nginx Proxy Manager), and building/running the binary
  directly.
- **[Configuration](Configuration.md)** — every `SHORTR_*` environment
  variable, grouped by concern, with defaults and validation rules.
- **[API Reference](API-Reference.md)** — REST endpoint map, auth model
  (session cookies vs. scoped API keys), pagination, and error envelope.
- **[MCP Server](MCP-Server.md)** — using Shortr from an AI agent over the
  Model Context Protocol.
- **[Deployment](Deployment.md)** — reverse proxies, TLS termination,
  systemd, horizontal-scaling caveats.
- **[Backup & Restore](Backup-and-Restore.md)** — automatic and on-demand
  SQLite backups, restoring, and PostgreSQL notes.
- **[Security](Security.md)** — the auth/session model, what's hardened by
  default, and what an operator is responsible for.
- **[Troubleshooting](Troubleshooting.md)** — common startup and runtime
  problems and how to diagnose them.
- **[Contributing](Contributing.md)** — running the project locally, tests,
  and how to submit changes.

## At a glance

| | |
|---|---|
| **Storage** | SQLite (default, embedded, pure-Go driver) or PostgreSQL |
| **Auth** | Local email+password, and/or OIDC/SSO, per-user scoped API keys |
| **Analytics** | Country/region/city (offline MaxMind GeoIP2), device/OS/browser, referrers, UTM, bot filtering, CSV export |
| **Notifications** | In-app always; optional SMTP email and/or self-hosted Gotify push |
| **Integrations** | REST API, OpenAPI 3 spec at `/api/v1/openapi.json`, an MCP server for AI agents |
| **Deployment** | Single binary, Docker image, Docker Compose, systemd unit |
