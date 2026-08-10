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
# securemode is forced to at least "hardened" while OIDC is active; "strict" is opt-in

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
- Nakama peers keep authenticating with the Nakama host password header.
- Non-browser clients (e.g. the mobile build) cannot perform the cookie flow;
  point them at a password-mode server instead.

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
  account, settings). Do not allowlist accounts you would not hand the server to;
  consider `securemode = "strict"` to restrict extension installation and other
  privileged local actions.
- The built-in self-signed TLS (`[server.tls]`) remains available as a fallback,
  but a reverse proxy with real certificates is the recommended setup.
