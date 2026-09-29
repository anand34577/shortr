# Android App

The Shortr app lets you shorten, manage and follow your links from your
phone. It talks to your own server through the same API as the web console;
there is no cloud service in between.

## What it does

- Dashboard with clicks, visitors, top links, recent clicks and a chart for
  the last 24 hours, 7, 30 or 90 days
- Create links with a custom alias (checked as you type), title, tags, expiry,
  click limit, password, and temporary or permanent redirect
- Search, filter (active, disabled, trash) and sort your links; copy, share,
  disable, delete and restore them
- Per-link stats: countries, referrers, devices, browsers and systems
- QR code for any link, ready to share as an image
- **Share → Shortr** from any app shortens the shared link
- Long-press the launcher icon for a **New link** shortcut
- Optional app lock with fingerprint, face or screen lock
- Light, dark or system theme, with optional Material You colours

## Install

1. Download `shortr-android-<version>.apk` from the
   [Releases](https://github.com/anand34577/shortr/releases) page. Check it
   against `SHA256SUMS-android` if you like.
2. Open it on your phone. Android asks you to allow installs from your
   browser or file manager once.

Android 8.0 or newer is required. The app works without Google Play
services.

## Connect to your server

The app needs to reach your server's **API**. What address to use depends
on how you expose Shortr (see
[Configuration → Exposure](Configuration.md#exposure-public-vs-private-listener)):

| Your setup | Address in the app |
|---|---|
| One port for everything | Your public URL, e.g. `https://sho.rt` |
| Split ports, public API on (the default) | Your public URL, e.g. `https://sho.rt` (works anywhere) |
| Split ports with `SHORTR_PUBLIC_API=false` | The console address on your LAN/VPN, e.g. `http://10.0.0.5:8081` (works only on that network) |

### Option 1: pairing code (easiest)

1. In the web console, open **Settings → API keys → New key**. Name it after
   the phone and tick `links:read`, `links:write` and `stats:read`. The app
   never needs `admin:*`.
2. The key is shown once, together with a QR code.
3. In the app, tap **Scan pairing code** and point the camera at it.

The code contains your server's public URL and the key. If you'd rather use
a VPN address, pair with the code, then disconnect and connect again with
Option 2.

### Option 2: address and API key

Tap **Enter server address**, type the address, then paste the API key.

### Option 3: sign in with Keycloak (or another OIDC provider)

Shortr can accept access tokens that your identity provider issues to the
app, so nobody has to copy keys around.

1. In Keycloak, create a new client for the app:
   - **Client ID**: `shortr-android` (any name works)
   - **Client authentication**: off (it's a public client)
   - **Standard flow**: on; everything else off
   - **Valid redirect URIs**: `io.github.anand34577.shortr:/oauth2redirect`
   - Under **Advanced**, set **Proof Key for Code Exchange Code Challenge
     Method** to `S256`.
2. On the Shortr server set
   `SHORTR_OIDC_API_AUDIENCES=shortr-android` and restart it.
3. Sign in to the **web console** with SSO once. This links your Keycloak
   account to your Shortr account; the app never creates accounts.
4. In the app, enter the server address. A **Sign in with …** button
   appears; tap it and sign in in the browser window.

The app refreshes tokens on its own. Tokens get the same access as a
`links:*` + `stats:read` API key, never admin.

## Security notes

- The key or tokens are encrypted with a key held in the Android Keystore
  and are excluded from backups, so copying the phone's data gives nothing
  usable. Removing the phone's screen lock can wipe that key; the app then
  asks you to connect again.
- **http** addresses are allowed so LAN servers work. The app warns you when
  a public address isn't using https.
- Certificates from a CA you installed yourself (Android **Settings →
  Security → Encryption & credentials → Install a certificate → CA
  certificate**) are trusted, so a private CA or step-ca works.
- **Disconnect** removes the key from the phone only. Revoke it in the web
  console if the phone is lost.

## Troubleshooting

| Message | What to check |
|---|---|
| "No Shortr API at this address" | Wrong address, or the server runs with `SHORTR_PUBLIC_API=false` and you used the public URL. |
| "Can't find that server" / "isn't answering" | You're off your VPN, or the port isn't reachable from the phone. |
| "Secure connection failed" | Self-signed or private-CA certificate that isn't installed on the phone. |
| "Your access to this server has ended" | The key was revoked or expired, or your user was disabled. Connect again. |
| SSO: "the server didn't accept the token" | `SHORTR_OIDC_API_AUDIENCES` doesn't contain the app's client ID, or you haven't signed in to the web console with SSO yet. |
| SSO button missing | The server has no `SHORTR_OIDC_API_AUDIENCES`, or SSO is disabled. |

## Building it yourself

See [android/README.md](../../android/README.md).
