# Running Shortr behind Nginx Proxy Manager

This gets Shortr on your own domain with a Let's Encrypt certificate, using
[Nginx Proxy Manager](https://nginxproxymanager.com/) (NPM) for TLS
termination — Shortr itself only ever speaks plain HTTP on the internal
Docker network.

## 1. Start NPM and Shortr on a shared network

```bash
docker network create npm-proxy   # skip if it already exists
cp .env.example .env              # set SHORTR_BASE_URL, SHORTR_SECRET_KEY
docker compose -f deploy/docker-compose.npm.yml up -d
```

Open the NPM admin UI at `http://<host>:81` (default login is printed in
`docker compose logs npm` on first start — change it immediately).

## 2. Add the proxy host

In NPM: **Proxy Hosts → Add Proxy Host**

| Field | Value |
|---|---|
| Domain Names | `sho.rt` (your domain) |
| Scheme | `http` |
| Forward Hostname / IP | `shortr` (the compose service name) |
| Forward Port | `8080` |
| Cache Assets | off (Shortr sets its own cache headers) |
| Block Common Exploits | on |
| Websockets Support | off (not used, harmless either way) |

**SSL tab**: request a new Let's Encrypt certificate, enable **Force SSL**,
**HTTP/2 Support**, and **HSTS Enabled**.

## 3. Point Shortr at the real client IP

NPM sits in front of Shortr, so every request Shortr sees comes from NPM's
container IP unless you tell it to trust NPM's forwarding headers. Find
NPM's subnet:

```bash
docker network inspect npm-proxy --format '{{(index .IPAM.Config 0).Subnet}}'
```

Set that as `SHORTR_TRUSTED_PROXIES` in `.env` (the compose file already
defaults to the typical `172.18.0.0/16`, but Docker doesn't guarantee that
subnet — verify it). Without this, every visitor's IP in your analytics will
be NPM's own address, and per-IP rate limiting will lump all visitors into
one bucket.

`SHORTR_BASE_URL` must be the public `https://` URL — this is what's used to
build short links and to decide whether cookies get the `Secure` flag, and
is authoritative over any `X-Forwarded-Host` header (so a spoofed header
can't redirect users to build phishing-looking short links).

## 4. Advanced NPM config (optional)

In the proxy host's **Advanced** tab, `deploy/nginx-advanced.conf` has two
optional tweaks:

- Pass through a client-generated request id if you have one upstream.
- Disable proxy buffering so large CSV click exports stream instead of
  buffering entirely in NPM's memory first.

## 5. Health checks

Point any external uptime monitor (e.g. Uptime Kuma) at
`https://sho.rt/healthz` — it returns `200` as long as the process is alive,
even mid-database-outage, so it reflects "can this monitor still resolve
links from cache" rather than "is the database currently reachable". For the
stricter "is Shortr fully operational" check, use `/readyz` instead, which
returns `503` if the database is unreachable.

## Other reverse proxies

The same two settings — `SHORTR_TRUSTED_PROXIES` and `SHORTR_BASE_URL` — are
all that's needed for Traefik, Caddy, or a Cloudflare Tunnel too:

- **Traefik**: trust Traefik's own container/network IP; `X-Forwarded-*` is
  set by default.
- **Caddy**: same; Caddy sets `X-Forwarded-For` automatically via
  `reverse_proxy`.
- **Cloudflare Tunnel**: set `SHORTR_REAL_IP_HEADER=CF-Connecting-IP` and
  trust `cloudflared`'s local container IP (or the tunnel's loopback address
  if run as a sidecar).
