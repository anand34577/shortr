# Installation

## Option 1 — Docker Compose (recommended)

Requires Docker and Docker Compose.

```bash
git clone https://github.com/<your-org>/shortr.git
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

Grab the archive for your OS/arch from the
[Releases page](../../../../releases) (built automatically by CI on every
tagged version — see [.github/workflows/release.yml](../../.github/workflows/release.yml)).
Verify the checksum against `SHA256SUMS` in the same release, then:

```bash
tar -xzf shortr-linux-amd64.tar.gz   # or unzip the .exe on Windows
export SHORTR_BASE_URL=https://links.example.com
./shortr
```

On first run with no users in the database, visit `/app/setup` to create the
first admin account (or set `SHORTR_ADMIN_EMAIL` / `SHORTR_ADMIN_PASSWORD`
to seed one non-interactively — see [Configuration](Configuration.md)).

## Option 3 — systemd (bare metal / VM)

```bash
sudo useradd --system --home /var/lib/shortr --shell /usr/sbin/nologin shortr || true
sudo mkdir -p /opt/shortr /var/lib/shortr
sudo cp shortr /opt/shortr/shortr
sudo cp deploy/shortr.service /etc/systemd/system/shortr.service
sudo systemctl daemon-reload
sudo systemctl enable --now shortr
```

Edit `/etc/systemd/system/shortr.service` (or drop an
`/etc/systemd/system/shortr.service.d/override.conf`) to set your
`SHORTR_*` environment variables — see [Configuration](Configuration.md).
The shipped unit already sets `DynamicUser=yes`, `NoNewPrivileges=yes`,
`ProtectSystem=strict`, and a `Restart=always` policy.

## Option 4 — build from source

Requires Go 1.25+ and Node 20+.

```bash
git clone https://github.com/<your-org>/shortr.git
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
