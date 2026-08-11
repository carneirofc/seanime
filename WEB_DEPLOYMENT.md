# Exposing Seanime on the web

Seanime can be safely exposed to the internet when OIDC login is configured.
Without it, keep the server bound to `127.0.0.1` (default) or protected by the
server password on a trusted network only.

## Recommended topology

Reverse proxy (Caddy, Traefik, nginx) with real TLS in front of a loopback-bound
Seanime, plus an OIDC identity provider (Keycloak, Authelia, Pocket ID, Dex,
Google, ...).

```toml
# <datadir>/config.toml
[server]
host = "127.0.0.1"                       # only the proxy reaches the server
port = 43211
trustedproxies = ["127.0.0.1"]           # the reverse proxy's address(es)
externalurl = "https://seanime.example.com"
# securemode defaults to "strict" while OIDC is active; set "hardened" to opt out

[server.oidc]
issuerurl = "https://idp.example.com"    # discovery at <issuer>/.well-known/openid-configuration
clientid = "seanime"
clientsecret = "..."                     # or env SEANIME_OIDC_CLIENT_SECRET
allowedsubjects = ["248289761001"]       # `sub` claims (recommended: stable across renames)
allowedusernames = ["carneirofc"]        # matched case-insensitively against usernameclaim
# usernameclaim = "preferred_username"   # default; falls back to "email"
# providername = "SSO"                   # login button label
# sessionttldays = 30                    # sliding expiry
# sessionmaxdays = 90                    # absolute cap
```

Register the client at your IdP with redirect URI:

```
https://seanime.example.com/api/v1/auth/oidc/callback
```

At least one of `allowedsubjects` / `allowedusernames` must be non-empty — the
server refuses to start with an empty allowlist. On a username-only match the
server logs the account's `sub` so you can pin it.

## What OIDC mode changes

- The server password is **ignored** entirely; browser access requires an IdP session.
- Unauthenticated clients only receive a minimal login shell at `/login` — the
  app bundle and static/media directories are session-gated.
- Sessions are server-side; the browser holds a `__Host-` HttpOnly cookie.
  Restarting the server keeps sessions (SQLite) but rotates media query tokens.
- Secure mode defaults to **`strict`** (was `hardened`). In strict mode,
  filesystem browsing, external process/player launch, extension installation,
  and library/source path changes are restricted to a **trusted local origin**,
  and outbound fetches (image/stream proxy) cannot reach private/loopback
  addresses. A remote authenticated (or compromised) session therefore cannot
  browse the host or pivot into your internal network. Manage those actions from
  a local session (e.g. an SSH tunnel to `127.0.0.1`), or set
  `securemode = "hardened"` to allow them remotely.
- Nakama peers keep authenticating with the Nakama host password header.
- Non-browser clients (e.g. the mobile build) cannot perform the cookie flow;
  point them at a password-mode server instead.

## Hardening checklist

For an internet-facing deployment:

- [ ] **Front it with a TLS-terminating reverse proxy** (Caddy/Traefik/nginx).
      See `example.Caddyfile` and `docker-compose.example.yml`.
- [ ] **Do not expose the app port to the host/internet.** Bind loopback on bare
      metal, or in Docker keep the port on an internal-only network reachable
      solely by the proxy (the compose example does this).
- [ ] **Keep `securemode = "strict"`** (the OIDC default) unless you need remote
      admin of filesystem/exec actions.
- [ ] **Set `externalurl`** to your canonical `https://` URL and **`trustedproxies`**
      to the proxy's address, so secure cookies and client-IP rate limits work.
- [ ] **Supply the OIDC client secret via `SEANIME_OIDC_CLIENT_SECRET`**, not in
      `config.toml`.
- [ ] **Set `allowedsubjects`** (preferred) and/or `allowedusernames`; the server
      refuses to start with an empty allowlist. Only allowlist accounts you would
      hand the whole server to — they share one Seanime identity.
- [ ] **Restrict data-dir permissions** (`0700`, owned by the service user). It
      holds the SQLite DB, sessions, logs, and installed extensions.
- [ ] **Treat installed extensions as trusted code.** They run JavaScript with
      network access; strict mode limits installation to local origins.
- [ ] **Keep the media library read-only** to the server if it never writes there.

## Signing in with GitHub

GitHub's OAuth2 is not OIDC (no ID token, no discovery document). Front it with
an IdP that bridges it, e.g. [Dex](https://dexidp.io) with the GitHub connector
or Authelia/Keycloak brokering GitHub — then point `issuerurl` at that IdP.

## Local development / testing

```toml
[server]
externalurl = "http://localhost:43211"

[server.oidc]
# ...
allowinsecure = true    # permits http and non-__Host- cookies — never in production
```

A quick local IdP: run Dex or Pocket ID in Docker with redirect URI
`http://localhost:43211/api/v1/auth/oidc/callback`.

## Notes

- If the IdP is unreachable at boot, the server still starts; login returns
  503 until discovery succeeds (retried on demand).
- All allowlisted accounts share the single Seanime identity (library, AniList
  account, settings). Do not allowlist accounts you would not hand the server to.
  Extension installation and other privileged local actions are already
  restricted by the default `securemode = "strict"`.
- The built-in self-signed TLS (`[server.tls]`) remains available as a fallback,
  but a reverse proxy with real certificates is the recommended setup.

## Docker

A self-contained image and a reverse-proxy-fronted stack are provided:

- `server.Dockerfile` — builds the web bundle and a static, non-root server image
  with `ffmpeg` for transcoding and a `/api/v1/status` healthcheck.
- `docker-compose.example.yml` — Seanime on an internal-only network behind Caddy;
  the app port is never published to the host.
- `config.example.toml` — a starting `config.toml` for this topology.
- `example.Caddyfile` — automatic-HTTPS reverse proxy to the app.

```sh
mkdir -p ./data && sudo chown -R 10001:10001 ./data   # container runs as uid 10001
cp config.example.toml ./data/config.toml     # then edit [server.oidc]
export SEANIME_OIDC_CLIENT_SECRET=...          # keep the secret out of config.toml
docker compose -f docker-compose.example.yml up -d
```

The image runs as a non-root user (uid 10001), so a bind-mounted data dir must
be writable by it (the `chown` above). Named volumes are chowned automatically.
