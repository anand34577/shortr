# API Reference

Full request/response schemas are published live at
`GET /api/v1/openapi.json` (OpenAPI 3) — import it into Postman/Insomnia or
generate a client. This page is the human-readable map: what exists, how
auth works, and the conventions every endpoint follows.

## Authentication

Two ways to authenticate, both resolved once per request and available to
every handler:

1. **Session cookie** (`shortr_session`) — set on login/register/OIDC
   callback. Used by the web UI. Mutating requests additionally require the
   `X-CSRF-Token` header (double-submit, checked against the session) and a
   matching `Origin` header.
2. **API key** (`Authorization: Bearer sk_...`) — created in
   Settings → API Keys, or via `POST /api/v1/apikeys`. Each key carries a
   set of **scopes**:

   | Scope | Grants |
   |---|---|
   | `links:read` | List/get links, QR codes, `links/check` |
   | `links:write` | Create/update/delete/restore links, bulk import, preview |
   | `stats:read` | Analytics, breakdowns, CSV export, IP lookup tool |
   | `admin:*` | Everything under `/api/v1/users`, `/api/v1/settings`, `/api/v1/audit`, `/api/v1/admin/*` — **only** meaningful for a key belonging to an admin user, and required in addition to `u.IsAdmin()` for an API key to reach any admin route. |

   API keys cannot manage other API keys, change the caller's password, or
   list/revoke sessions — those require a session cookie (`requireSession`
   routes) to limit the blast radius of a leaked key.
3. **SSO access token** (`Authorization: Bearer <JWT>`), only when
   `SHORTR_OIDC_API_AUDIENCES` is set. A mobile app signs in with your
   provider (e.g. Keycloak, authorization code + PKCE) and sends the access
   token. It must be signed by the configured issuer, unexpired, and carry
   an allowed `azp` or `aud`; ID tokens are refused. It acts as the linked
   user with `links:read`, `links:write` and `stats:read`.

In split mode (`SHORTR_ADMIN_LISTEN`), the public listener accepts only
methods 2 and 3; cookies work on the admin listener.

### Pairing a mobile app

When a key is created in the web console, the one-time reveal dialog also
shows a QR code encoding:

```
shortr://connect?server=<SHORTR_BASE_URL, url-encoded>&key=<sk_…, url-encoded>
```

An app registers the `shortr` scheme, reads `server` and `key`, and calls
`GET {server}/api/v1/me` to confirm the pairing.

## Conventions

- **Base path:** `/api/v1`.
- **Pagination:** cursor-based (`?cursor=...&limit=...`), response shape
  `{ "items": [...], "nextCursor": "..." }`. `limit` is always clamped
  server-side (typically ≤100) regardless of what's requested.
- **Errors:** a consistent envelope —
  ```json
  { "error": { "code": "NOT_FOUND", "message": "...", "requestId": "..." } }
  ```
  Use `requestId` when reporting a bug; it's echoed in server logs.
- **Idempotency:** mutating endpoints that create resources accept an
  `Idempotency-Key` header; a retried request with the same key and body
  returns the original result instead of creating a duplicate.
- **Rate limits:** `SHORTR_RATE_LIMIT_API` applies to every `/api/*` and
  `/mcp` request, keyed by user/API-key id (falling back to IP). A `429`
  includes `Retry-After`.

## Endpoint map

### Account (`/api/v1/me`, session-cookie-oriented)
`GET/PATCH /me` · `PUT /me/password` · `DELETE /me` · `GET /me/sessions` ·
`DELETE /me/sessions/{id}` · `GET /me/identities` ·
`DELETE /me/identities/{id}`

### Notifications
`GET /notifications` · `PATCH /notifications/{id}/read` ·
`POST /notifications/read-all` ·
`GET/PUT /notifications/preferences`

### Links
`POST /links` · `GET /links` · `POST /links/bulk` · `GET /links/check` ·
`POST /links/preview` · `GET/PATCH/DELETE /links/{id}` ·
`POST /links/{id}/restore` · `GET /links/{id}/qr.png` (and `.svg`) ·
`GET /links/{id}/stats` · `GET /links/{id}/clicks` ·
`GET /links/{id}/clicks/export` (CSV, formula-injection-safe)

### Stats & tools
`GET /stats/overview` · `GET /stats/recent` ·
`GET /tools/ip-lookup/{ip}`

### API keys (session-cookie only)
`POST/GET /apikeys` · `DELETE /apikeys/{id}`

### Admin (`admin:*` scope + `IsAdmin()` required)

`GET /api/v1/audit` accepts `action` (prefix match, so `user.login_` finds
failed and locked-out sign-ins), `actor` (user id), `target` (id),
`cursor` and `limit`.
`POST/GET /users` · `GET/PATCH/DELETE /users/{id}` ·
`POST /users/{id}/reset-password` · `DELETE /users/{id}/sessions` ·
`DELETE /users/{id}/identities/{iid}` · `GET/PUT /settings` ·
`GET /audit` · `GET /admin/links` · `POST /admin/links/{id}/purge` ·
`GET /admin/system` · `POST /admin/backup`

### Public / unauthenticated
`GET /healthz` · `GET /readyz` · `GET /metrics` (bearer-token-gated if
`SHORTR_METRICS_TOKEN` is set) · `GET /version` ·
`GET /api/v1/openapi.json` · `GET /robots.txt` ·
`GET/POST /auth/*` (setup, login, logout, register, OIDC start/callback) ·
the redirect hot path itself: `GET /{code}`.

See also: [MCP Server](MCP-Server.md) for the AI-agent-facing tool surface
built on top of this same auth model.
