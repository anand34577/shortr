# Installation

## Option 1 — Docker Compose (recommended)

Requires Docker and Docker Compose.

```bash
git clone https://github.com/anand34577/shortr.git
cd shortr
cp .env.example .env      # set SHORTR_BASE_URL at minimum
docker compose up -d
```

Open `http://localhost:8080/app/setup` once to create the first admin
account (this route disables itself after the first admin exists).

**PostgreSQL instead of SQLite:**

```bash
docker compose -f deploy/docker-compose.postgres.yml up -d
```

**Behind Nginx Proxy Manager** (TLS termination, your own domain):

```bash
docker compose -f deploy/docker-compose.npm.yml up -d
```

See [deploy/NGINX_PROXY_MANAGER.md](../../deploy/NGINX_PROXY_MANAGER.md) for
the full walkthrough, including correct client-IP forwarding so analytics
and rate limiting see real visitor IPs, not the proxy's.

## Option 2 — Prebuilt binary

Download the binary for your platform from the
[Releases page](https://github.com/anand34577/shortr/releases) and check it
against the `SHA256SUMS` file from the same release:

```bash
curl -LO https://github.com/anand34577/shortr/releases/latest/download/shortr-linux-amd64
curl -LO https://github.com/anand34577/shortr/releases/latest/download/SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
chmod +x shortr-linux-amd64
SHORTR_BASE_URL=https://links.example.com ./shortr-linux-amd64
```

Available files: `shortr-linux-amd64`, `shortr-linux-arm64`,
`shortr-darwin-amd64`, `shortr-darwin-arm64` and `shortr-windows-amd64.exe`.

On first run with no users in the database, visit `/app/setup` to create the
first admin account (or set `SHORTR_ADMIN_EMAIL` / `SHORTR_ADMIN_PASSWORD`
to seed one non-interactively — see [Configuration](Configuration.md)).

## Option 3 — systemd (bare metal / VM)

```bash
sudo install -m 0755 shortr-linux-amd64 /usr/local/bin/shortr
sudo mkdir -p /etc/shortr
sudoedit /etc/shortr/env        # SHORTR_BASE_URL=https://links.example.com
sudo cp deploy/shortr.service /etc/systemd/system/shortr.service
sudo systemctl daemon-reload
sudo systemctl enable --now shortr
```

Put your `SHORTR_*` variables in `/etc/shortr/env`, one `KEY=value` per
line — see [Configuration](Configuration.md). The unit runs as a dynamic
user, keeps its data in `/var/lib/shortr`, and restarts on failure.

## Option 4 — build from source

Requires Go 1.26+ and Node 20+.

```bash
git clone https://github.com/anand34577/shortr.git
cd shortr
make build          # builds web/ then the Go binary into bin/shortr
SHORTR_BASE_URL=http://localhost:8080 ./bin/shortr
```

`make release` cross-compiles linux/darwin/windows × amd64/arm64 into
`dist/` with a `SHA256SUMS` file — the same thing CI does on a tagged push.

## Verifying it's up

```bash
curl -fsS http://localhost:8080/healthz   # liveness
curl -fsS http://localhost:8080/readyz    # readiness (DB reachable)
shortr config check                       # validate config without starting the server
shortr version
```
