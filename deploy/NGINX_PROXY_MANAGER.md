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

## 6. Split exposure: public redirects via Cloudflare Tunnel, admin console via VPN only

Goal: anyone on the internet can follow a short link on your public
hostname, but `/app`, `/api`, `/auth`, `/mcp` are only reachable from your
own VPN. This section uses two placeholders — swap in whatever's actually
yours, they don't need to be related or even on the same domain:

- `<public-host>` — the domain you already point at this box through your
  Cloudflare Tunnel (what sections 1-5 above call `sho.rt`; that was always just
  the example placeholder in this doc, not a literal requirement).
- `<private-host>` — any hostname/subdomain you pick for the admin
  console. Doesn't need a public DNS record at all (see section 6.3).

These instructions reference Netbird since that's what you have running,
but the pattern — a second hostname, an NPM Access List by CIDR, a DNS-01
cert — works the same with any VPN that gives you a private subnet
(Tailscale, WireGuard, etc.), so it carries over if that ever changes. One
shortr backend, two NPM proxy hosts, each with different reach. Local dev
(`docker compose up` against the root `docker-compose.yml`, or `make run`)
is untouched by any of this — it's a separate stack.

### 6.0 What you need before starting

- shortr, NPM and (optionally) a Netbird sidecar all reachable from each
  other on the `npm-proxy` Docker network — already true if you followed
  section 1.
- `cloudflared` already running and connected (you have this).
- Netbird already installed and joined on the docker host, and on any
  device that should reach `<private-host>` (you have this too).
- A DNS provider account with API access for DNS-01 certificate issuance
  (Cloudflare's own API token works if your domain is on Cloudflare —
  NPM's DNS Provider settings support it and several others).
- Admin access to your NPM instance and your Netbird admin panel.

Architecture once this section is done:

```
                              ┌─────────────────────┐
Internet ──▶ Cloudflare ──▶ cloudflared ──▶ NPM (<public-host>) ──▶ redirects
  (anyone)      Tunnel                        │  nginx: only "/" + /healthz
                                               │  pass through, everything
                                               │  else -> 404
                                               │
Netbird peers ──▶ (mesh network) ──▶ NPM (<private-host>) ──▶ shortr:8080
 (your devices)                        │  Access List: Netbird CIDR only    (full app)
                                        │  everything else rejected before
                                        │  it reaches shortr
```

Both `<public-host>` and `<private-host>` proxy to the *same* `shortr:8080`
backend — it's one instance, one database, reached two different ways.

### 6.1 Your existing Cloudflare Tunnel

Nothing to add here — your `cloudflared` is already running and managed
outside this repo. In its tunnel config (Cloudflare dashboard's **Public
Hostname** tab, or your local `config.yml`/ingress rules, whichever you
manage it with), confirm `<public-host>` points at wherever this NPM
container is reachable from that tunnel (e.g. `http://npm:80` if
`cloudflared` shares the `npm-proxy` Docker network, or the host's LAN
address:port if it runs elsewhere). That's the only place traffic for the
hostname is routed from — the tunnel forwards nothing else on your network.

**Verify:** `curl -I https://<public-host>/healthz` from any machine
outside your network should return `200`.

### 6.2 Public NPM proxy host — redirects only

**Proxy Hosts → Add Proxy Host**: Domain `<public-host>`, forward to
`shortr:8080` as in section 2 (this can be your existing proxy host — just add
the Advanced-tab block below to it, no need to recreate it). In its
**Advanced** tab, paste `deploy/nginx-public-redirect-only.conf` — it
`return 404`s `/app`, `/api`, `/auth`, `/mcp`, `/metrics` at the nginx
layer, before the request ever reaches shortr. This means even if
shortr's own `SHORTR_UI_ENABLED` were left on (it should be — you want
the console reachable via VPN), this hostname physically can't serve it.
Click **Save**, and give NPM a few seconds to reload nginx.

**Verify** (again from outside your network, e.g. mobile data with wifi
off, or `curl` from a cloud VM — testing from inside your own network can
give a false pass if your router/DNS routes you around the tunnel):

```bash
curl -I https://<public-host>/app/           # expect 404
curl -I https://<public-host>/api/v1/me      # expect 404
curl -I https://<public-host>/healthz        # expect 200
```

If `/app/` doesn't 404, double check the Advanced-tab block actually saved
(NPM sometimes needs the proxy host re-saved for a custom-location change
to take effect) and that you edited the right proxy host.

### 6.3 Private NPM proxy host — full access, VPN-only

1. Look up the CIDR your Netbird peers actually get from your own Netbird
   admin panel (**Network Routes / Peers**) — don't assume a default, it
   depends on your setup.
2. **Proxy Hosts → Add Proxy Host**: Domain `<private-host>` — pick any
   hostname you like on any domain you control, it's never advertised
   anywhere public — forward to `shortr:8080`, same as the public host
   otherwise.
3. **Access List** (NPM → Access Lists → Add): restrict by IP, allow only
   your Netbird CIDR. Attach it to this proxy host. This is the actual
   enforcement — it checks the request's source IP regardless of which
   network interface it arrived on, so it holds even if 80/443 stay bound
   to all interfaces.
4. **SSL tab**: request the certificate via **DNS Challenge** (add your
   DNS provider's API token in NPM's DNS provider settings) instead of
   HTTP-01 — this only needs a `_acme-challenge` TXT record, not a
   publicly reachable webserver, so it works even though this hostname is
   deliberately not meant to resolve/route publicly.
5. Resolve `<private-host>` for your devices: Netbird → **DNS** →
   **Nameserver Groups**, add a custom domain mapping it to the docker
   host's Netbird peer address. (If your Netbird plan/version doesn't
   support that, a per-device `/etc/hosts` entry pointing at the host's
   Netbird IP works identically — just less automatic.) Don't publish a
   public DNS record for it.

**Verify:**

```bash
# on the Netbird network:
curl -I https://<private-host>/app/          # expect 200

# off the Netbird network (mobile data, VPN disconnected):
curl -I https://<private-host>/app/          # expect connection refused/
                                              # timeout (Access List) or, if
                                              # DNS doesn't resolve there at
                                              # all, that's fine too
```

If the on-VPN check fails with a TLS error, the DNS-01 cert probably
hasn't issued yet — check **NPM → SSL Certificates** for its status. If it
fails with connection refused even on the VPN, re-check the Access List's
CIDR against what Netbird's admin panel actually assigned your peer (`ip a`
on the peer, or the Netbird admin UI's peer list, will show the real
address — CIDRs from memory or documentation elsewhere are unreliable).

### 6.4 Everything else, before going live

- Finish `/auth/setup` (or set `SHORTR_ADMIN_EMAIL` / `SHORTR_ADMIN_PASSWORD`
  for non-interactive bootstrap) **before** `<public-host>` goes public via
  the tunnel — setup is unauthenticated by design (there's no admin yet)
  and only closes once the first account exists. `/auth/setup` is only
  reachable from `<private-host>` once sections 6.2 and 6.3 are live, so do
  this from there, or do it first and confirm afterwards with
  `curl https://<public-host>/auth/status` → `"setupRequired": false`.
- Set `SHORTR_REAL_IP_HEADER=CF-Connecting-IP` and add cloudflared's/NPM's
  container IP (verified in section 3) to `SHORTR_TRUSTED_PROXIES` — otherwise
  every request is attributed to the tunnel's address and per-IP rate
  limiting collapses to one shared bucket.
- Confirm `SHORTR_REGISTRATION=closed` (the default) — the public host has
  no route to `/auth/register` anyway per the redirect-only rule in section 6.2, but keep it closed as a
  second layer in case that ever changes.
- Keep NPM's admin port (`:81`) off both hostnames' DNS entirely — reach it
  over the Netbird network at the host's private IP, same as
  `<private-host>`.
- Android app: install the Netbird client on the phone, join the same
  network, and have the app call `<private-host>` directly with a scoped
  API key (`POST /api/v1/apikeys` — mint it once from the console over the
  VPN, scopes `links:read`/`links:write`/`stats:read`, never `admin:*`
  unless actually needed). This keeps the API off the public internet
  entirely, which is a smaller attack surface than exposing it and relying
  on the API key alone.
- `SHORTR_UI_ENABLED` (added for this project, see `.env.example`) stays
  at its default `true` for this setup — you *want* `/app` served, just
  only reachable through `<private-host>`. Set it `false` instead if you'd
  rather run a dedicated public-only instance with zero UI code path as
  extra defense-in-depth; not required when section 6.2's nginx block already
  enforces the same thing at the edge.

### 6.5 Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `<public-host>/app/` returns the app instead of 404 | Advanced-tab snippet not saved on the right proxy host, or NPM didn't reload | Re-open the public proxy host, re-paste the block, Save again |
| Login on `<private-host>` immediately logs you back out / cookie doesn't stick | `SHORTR_BASE_URL` doesn't match the hostname you're actually browsing, or `SHORTR_COOKIE_SECURE` mismatch | `SHORTR_BASE_URL` should be the `https://` public-facing URL you use most (usually `<public-host>`); as long as both proxy hosts terminate TLS, cookies work from either — check you're on `https://`, not `http://`, for `<private-host>` |
| Analytics show every click from the same IP | `SHORTR_TRUSTED_PROXIES` doesn't include NPM's actual subnet, or `SHORTR_REAL_IP_HEADER` is wrong for the path traffic takes | Re-run the `docker network inspect` command in section 3; set `CF-Connecting-IP` only for traffic that actually passes through Cloudflare |
| `<private-host>` unreachable even while connected to Netbird | Access List CIDR doesn't match your peer's real address, or DNS for `<private-host>` isn't resolving to the right IP | `ip a` (or your OS equivalent) on the peer to get the real address; `nslookup <private-host>` to confirm resolution |
| Cert for `<private-host>` stuck "pending" | DNS provider API token in NPM is missing/wrong scope, or the TXT record propagation is slow | Check **NPM → SSL Certificates → (cert) → View Logs**; DNS-01 can take a few minutes to propagate, retry after 5 |
| `/auth/setup` returns "setup already complete" when you expected otherwise | Someone (possibly a scanner, if this ran before the redirect-only rule in section 6.2 was in place) already created the first account | Check **Admin → Users**; if it's not you, rotate `SHORTR_SECRET_KEY` and every session, and re-verify the redirect-only rule in section 6.2 was live before `<public-host>` ever went public |

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
