# Security

## Good practice for operators

- **Put Shortr behind HTTPS.** It serves plain HTTP; terminate TLS in a
  reverse proxy. See [Deployment](Deployment.md).
- **Set `SHORTR_TRUSTED_PROXIES`** when you use a reverse proxy. Without it,
  visitor IPs, analytics and rate limits are all based on the proxy's
  address.
- **Set `SHORTR_SECRET_KEY` explicitly** if you move the data directory or
  run from more than one location. Otherwise a key is generated and stored in
  the data directory.
- **Keep private-network targets blocked.** `SHORTR_ALLOW_PRIVATE_TARGETS`
  is off by default. Turning it on lets every user create links to internal
  addresses.
- **Give API keys the smallest scopes they need**, especially keys used by
  scripts or AI assistants. `admin:*` is only needed for admin endpoints.
- **Protect `/metrics`** with `SHORTR_METRICS_TOKEN` if it is reachable from
  outside your monitoring network.
- **Keep secrets out of version control.** Use environment variables or an
  untracked `.env` file for SMTP, OIDC and database credentials.
- **Back up regularly.** See [Backup and Restore](Backup-and-Restore.md).
- **Keep the admin console off the public internet if you can.** If only
  the redirect hot path needs to be public, put `/app`, `/api`, `/auth`
  and `/mcp` behind a VPN-only hostname instead of exposing everything
  through the same public reverse-proxy path — see
  [deploy/NGINX_PROXY_MANAGER.md](../../deploy/NGINX_PROXY_MANAGER.md#6-split-exposure-public-redirects-via-cloudflare-tunnel-admin-console-via-vpn-only)
  section 6, or set `SHORTR_UI_ENABLED=false` on a dedicated public-only
  instance if you don't need the console reachable remotely at all.

## If a key or session leaks

Revoke it right away: **Settings → API keys** for keys, **Settings →
Security** for sessions. Admins can also end all of a user's sessions from
the user's page in the admin panel.

## Reporting a vulnerability

Please don't open a public issue. Email the maintainer or use GitHub's
private vulnerability reporting on the repository's Security tab, and include
steps to reproduce.
