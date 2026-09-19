# Shortr

[![CI](https://github.com/anand34577/shortr/actions/workflows/ci.yml/badge.svg)](https://github.com/anand34577/shortr/actions/workflows/ci.yml)
[![Release](https://github.com/anand34577/shortr/actions/workflows/release.yml/badge.svg)](https://github.com/anand34577/shortr/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A self-hosted URL shortener with built-in analytics. One binary, one
container, no external services required.

- Backend: Go, SQLite by default or PostgreSQL
- Frontend: React and TypeScript, embedded in the binary
- Docs: [Wiki](https://github.com/anand34577/shortr/wiki)

## Features

- **Short links** — custom or generated codes, expiry dates, click limits,
  password protection, tags, bulk import, QR codes, soft delete with restore
- **Analytics** — clicks over time, countries and cities (with an optional
  offline GeoIP database), devices, browsers, referrers, UTM campaigns, bot
  filtering, CSV export
- **Privacy controls** — store, anonymize, hash, or drop visitor IPs
- **Accounts** — local sign-in, single sign-on through any OIDC provider,
  registration modes, an admin panel for users and settings
- **Notifications** — in-app, plus optional email (SMTP) and Gotify push
- **Integrations** — REST API with scoped API keys, OpenAPI spec, and an MCP
  server for AI assistants
- **Operations** — health and Prometheus endpoints, automatic SQLite
  backups, works behind Nginx Proxy Manager, Traefik, Caddy or Cloudflare
  Tunnel

## Getting started

Build from source (Go 1.26+ and Node 20+):

```bash
git clone https://github.com/anand34577/shortr.git
cd shortr
make build
SHORTR_BASE_URL=http://localhost:8080 ./bin/shortr
```

Open `http://localhost:8080/app/setup` and create the first admin account.
On later runs the setup page is disabled.

For UI work, run the backend as above and start the frontend dev server in
a second terminal:

```bash
cd web
npm install
npm run dev
```

## Docker

```bash
cp .env.example .env      # set SHORTR_BASE_URL at minimum
docker compose up -d
```

Compose variants for other setups:

```bash
# PostgreSQL instead of SQLite
docker compose -f deploy/docker-compose.postgres.yml up -d

# Behind Nginx Proxy Manager
docker compose -f deploy/docker-compose.npm.yml up -d
```

Data lives in the `/data` volume. Images are also published to
`ghcr.io/anand34577/shortr` for each release.

## Deploying a release build

Download the binary for your platform from the
[Releases](https://github.com/anand34577/shortr/releases) page. Each release
includes a `SHA256SUMS` file.

### Linux (systemd)

```bash
curl -LO https://github.com/anand34577/shortr/releases/latest/download/shortr-linux-amd64
sudo install -m 0755 shortr-linux-amd64 /usr/local/bin/shortr
sudo curl -Lo /etc/systemd/system/shortr.service https://raw.githubusercontent.com/anand34577/shortr/main/deploy/shortr.service
sudo systemctl daemon-reload
sudo systemctl enable --now shortr
```

Put your `SHORTR_*` variables in `/etc/shortr/env` (one `KEY=value` per line). Use
`shortr-linux-arm64` on ARM boards.

### macOS and Windows

Download `shortr-darwin-arm64`, `shortr-darwin-amd64` or
`shortr-windows-amd64.exe`, set `SHORTR_BASE_URL`, and run it. On macOS run
`chmod +x` on the file first.

## Configuration

Everything is configured with `SHORTR_*` environment variables, and
`.env.example` lists the common ones. Invalid values stop the server at
startup with a clear message, and `shortr config check` validates your
setup without starting it. The full list is on the
[Configuration](https://github.com/anand34577/shortr/wiki/Configuration)
wiki page.

If you run behind a reverse proxy, set `SHORTR_TRUSTED_PROXIES` so visitor
IPs are read correctly. Use `shortr admin create` to add an admin from the
command line.

## API access, MCP, and audit logging

Create API keys under **Settings → API keys**. Each key gets only the scopes
you choose (`links:read`, `links:write`, `stats:read`, `admin:*`) and can
carry an expiry.

```bash
curl -H "Authorization: Bearer sk_..." https://links.example.com/api/v1/links
```

The OpenAPI spec is served at `/api/v1/openapi.json`.

Admins can turn on the MCP server under **Admin → Settings**. AI assistants
then manage links through the same scoped keys at `/mcp`. See the
[MCP](https://github.com/anand34577/shortr/wiki/MCP-Server) wiki page for
client setup.

Sensitive actions, such as user changes and settings updates, are recorded
in the audit log under **Admin → Audit**.

## License

MIT. See [LICENSE](LICENSE).
