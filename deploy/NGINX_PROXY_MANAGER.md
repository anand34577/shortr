# Running Shortr behind Nginx Proxy Manager

This gets Shortr on your own domain with a Let's Encrypt certificate, using
[Nginx Proxy Manager](https://nginxproxymanager.com/) (NPM) for TLS
termination — Shortr itself only ever speaks plain HTTP on the internal
Docker network.

## 1. Start NPM and Shortr on a shared network

```bash
docker network create npm-proxy   # skip if it already exists
cp .env.example .env              # set SHORTR_BASE_URL, SHORTR_SECRET_KEY
docker compose --env-file .env -f deploy/docker-compose.npm.yml up -d
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

## 6. Split exposure: public redirects via Cloudflare Tunnel, console via VPN only

Goal: anyone on the internet can follow a short link, but the web console,
sign-in and admin API are only reachable from your own network or VPN.

Shortr does this itself. Set `SHORTR_ADMIN_LISTEN` and it runs two
listeners from one process and one database:

| Listener | Env | Serves | Put it behind |
|---|---|---|---|
| Public (`:8080`) | `SHORTR_LISTEN` | Short-link redirects, link password pages, `/healthz`, and a token-only API (API keys or SSO access tokens, never cookies) | Cloudflare Tunnel |
| Private (`:8081`) | `SHORTR_ADMIN_LISTEN` | Everything: web console, sign-in/SSO, setup, admin API, `/metrics` | LAN / VPN only |

On the public listener `/app`, `/auth/*`, `/metrics`, `/version` and the
admin API return the same 404 as an unknown short code, and session
cookies are ignored, so even a stolen cookie is useless from outside.
`SHORTR_PUBLIC_API=false` removes the API from the public side entirely;
`SHORTR_PUBLIC_ADMIN_API=true` puts the admin endpoints back (for an admin
mobile app with an `admin:*` key; leave it off unless you need it).

Placeholders below: `<public-host>` is the domain your tunnel serves;
`<private-host>` is any name you pick for the console and never publish.
The steps mention Netbird, but any VPN that gives you a private subnet
(Tailscale, WireGuard, …) works the same way.

```
Internet ─▶ Cloudflare ─▶ cloudflared ─▶ (NPM <public-host>) ─▶ shortr:8080  redirects + token API
VPN peers ─────────────────────────────▶  NPM <private-host>  ─▶ shortr:8081  console + everything
                                           Access List: VPN CIDR only
```

### 6.1 Shortr settings

In `.env` (the npm compose file already wires these through):

```bash
SHORTR_BASE_URL=https://<public-host>
SHORTR_ADMIN_LISTEN=:8081
SHORTR_ADMIN_URL=https://<private-host>      # required with SSO: the callback returns here
SHORTR_REAL_IP_HEADER=CF-Connecting-IP       # tunnel traffic; VPN traffic falls back to X-Forwarded-For
SHORTR_TRUSTED_PROXIES=172.18.0.0/16         # from section 3
# SHORTR_ROOT_REDIRECT=https://example.com   # where https://<public-host>/ should go (default: 404)
```

### 6.2 Cloudflare Tunnel → public listener

In your tunnel's **Public Hostname** config, point `<public-host>` at
`http://npm:80` and add an NPM proxy host forwarding to `shortr:8080`. Or
skip NPM for the public side and point the tunnel straight at
`http://shortr:8080` if `cloudflared` shares the Docker network. Either way,
**never route the tunnel to port 8081.**

Optional defence in depth: paste `deploy/nginx-public-redirect-only.conf`
into the public proxy host's **Advanced** tab. Shortr already hides those
paths on this port; the snippet just stops such requests one hop earlier.
(It also blocks `/api`, so skip it if a mobile app uses the public API.)

**Verify** from outside your network (mobile data, wifi off; testing from
home can route around the tunnel and give a false pass):

```bash
curl -I https://<public-host>/healthz        # 200
curl -I https://<public-host>/app/           # 404
curl -I https://<public-host>/auth/status    # 404
curl -H "Authorization: Bearer sk_..." https://<public-host>/api/v1/links   # 200 with a valid key
```

### 6.3 Private NPM proxy host → admin listener

1. Find the CIDR your VPN peers actually get (Netbird admin panel →
   **Peers**). Don't guess.
2. **Proxy Hosts → Add**: domain `<private-host>`, forward to `shortr:8081`.
3. **Access Lists → Add**: allow only your VPN CIDR, deny all; attach it to
   this proxy host. This is the real enforcement: it checks the source IP
   whatever interface the request came in on.
4. **SSL**: use a **DNS Challenge** certificate (Cloudflare API token in
   NPM's DNS provider settings). It needs only a TXT record, so the name
   never has to resolve publicly.
5. Make `<private-host>` resolve on your devices: Netbird → **DNS** →
   **Nameserver Groups** (map it to the docker host's VPN address), or an
   `/etc/hosts` entry per device. Publish no public DNS record for it.

No proxy for the console at all? You can reach `http://<lan-ip>:8081`
directly on your LAN. Publish the port only on that interface
(`"192.168.1.10:8081:8081"` in compose), never on `0.0.0.0` of an
internet-facing host.

**Verify:** on the VPN, `curl -I https://<private-host>/app/` returns `200`;
off the VPN it should time out or fail to resolve.

### 6.4 Keycloak (or any OIDC provider)

In split mode, sign-in happens on the private listener, so register this
redirect URI on the client:

```
https://<private-host>/auth/oidc/callback
```

To let a mobile app call the public API with its own Keycloak login instead
of an API key, create a separate public client for the app (PKCE, no
secret) and list it in `SHORTR_OIDC_API_AUDIENCES=shortr-android`. Shortr
accepts access tokens whose `azp` or `aud` matches, maps them to the user
who has already signed in through SSO once, and grants `links:read`,
`links:write` and `stats:read`, never admin.

### 6.5 Before going live

- Create the first admin (or set `SHORTR_ADMIN_EMAIL`/`SHORTR_ADMIN_PASSWORD`)
  before the tunnel goes up. In split mode setup is only reachable on the
  private listener anyway.
- Keep `SHORTR_REGISTRATION=closed` unless you mean it.
- Keep NPM's own admin port (`:81`) off every public hostname.
- Mobile app: create a key under **Settings → API keys** and scan the
  pairing QR code that is shown once; it carries the public URL and the key.
  Prefer `links:*`/`stats:read` scopes over `admin:*`.

### 6.6 Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `<public-host>/app/` serves the app | Tunnel/NPM points at `:8081`, or `SHORTR_ADMIN_LISTEN` isn't set | Point the public route at `shortr:8080`; look for the `listening (public: …)` log line at startup |
| Sign-in on `<private-host>` doesn't stick | Browsing `http://` while `SHORTR_ADMIN_URL` is `https://` (or vice versa) | Make `SHORTR_ADMIN_URL` exactly the URL in your address bar |
| SSO lands on the public host and 404s | `SHORTR_ADMIN_URL` unset, or the callback isn't registered in Keycloak | Set it, and add `https://<private-host>/auth/oidc/callback` to the client |
| Every click/audit entry shows the same IP | `SHORTR_TRUSTED_PROXIES` doesn't cover NPM/cloudflared | Re-check section 3's `docker network inspect` |
| `<private-host>` unreachable on the VPN | Access List CIDR or DNS mapping is wrong | Compare with the peer's real address; `nslookup <private-host>` |
| Certificate stuck pending | DNS API token missing or wrong scope | **NPM → SSL Certificates → View Logs**; retry after propagation |

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
