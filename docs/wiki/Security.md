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
- **Keep the admin console off the public internet.** Set
  `SHORTR_ADMIN_LISTEN=:8081` and expose only the main port (8080) to the
  internet. The public port then serves redirects and a token-only API: no
  web console, no password sign-in, no setup page, no metrics, and session
  cookies are ignored. The console stays on your LAN/VPN. See
  [Configuration](Configuration.md#exposure-public-vs-private-listener) and
  [deploy/NGINX_PROXY_MANAGER.md](../../deploy/NGINX_PROXY_MANAGER.md) section 6.
- **Check the Exposure card** under **Admin → System** after deploying. It
  shows what each listener serves.
- **Review the audit log.** Sign-ins, failed sign-ins, lockouts,
  re-authentication, key changes, settings changes and link changes are
  recorded with IP, user agent and how the caller authenticated
  (web session, API key or SSO token). Filter by **Failed sign-ins** to spot
  password guessing.

## Two-factor sign-in

Accounts that sign in with a password can add a second step: a 6-digit
code from an authenticator app (Aegis, 2FAS, Google Authenticator,
1Password and so on). Turn it on under **Settings → Security**. SSO users
get two-factor sign-in from their identity provider instead.

- Setting it up, changing recovery codes and turning it off all ask for the
  password again.
- Each code works once. Ten one-time recovery codes are shown when you turn
  it on; keep them somewhere other than your phone.
- The TOTP secret is encrypted in the database with a key derived from
  `SHORTR_SECRET_KEY`. If you replace that key, everyone has to set up
  two-factor sign-in again.
- Failed codes are rate limited and locked out like failed passwords, and
  recorded in the audit log (`user.mfa_failed`).
- Admins can require it for everyone who signs in with a password: **Admin
  → Settings → Require two-factor sign-in**, or `SHORTR_MFA_REQUIRED=true`.
  Until they enrol, those users can only reach their own account settings.
  API keys keep working; revoke old ones if that matters to you.

**Lost phone and recovery codes?** An admin opens the user under **Admin →
Users** and clicks **Remove 2FA**. If that admin is you, run this on the
server (it works while Shortr is running):

```bash
docker exec shortr /shortr admin reset-mfa --email you@example.com
# or, without Docker
shortr admin reset-mfa --email you@example.com
```

## If a key or session leaks

Revoke it right away: **Settings → API keys** for keys, **Settings →
Security** for sessions. Admins can also end all of a user's sessions from
the user's page in the admin panel.

## Reporting a vulnerability

Please don't open a public issue. Email the maintainer or use GitHub's
private vulnerability reporting on the repository's Security tab, and include
steps to reproduce.
