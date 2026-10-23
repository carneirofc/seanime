# Deploying Seanime

How to run Seanime as a server, and how to expose it to the internet safely. For building
from source and the development loop, see [`../DEVELOPMENT_AND_BUILD.md`](../DEVELOPMENT_AND_BUILD.md).

| Path | What |
| --- | --- |
| [`config.example.toml`](config.example.toml) | Annotated `config.toml` for an internet-facing, OIDC-protected server |
| [`docker/`](docker/) | `server.Dockerfile`, `web.Dockerfile`, Compose stacks and Caddyfiles ([Docker and Podman](#docker-and-podman)) |
| [`k8s/`](k8s/) | Reference Kubernetes manifests ([`k8s/README.md`](k8s/README.md)) |
| [`../packaging/linux/`](../packaging/linux/) | Bare-metal install with a hardened systemd unit ([systemd](#systemd-bare-metal)) |

## Exposing Seanime on the web

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
# capabilities = []                      # privileged actions, all denied by default here

[server.oidc]
issuerurl = "https://idp.example.com"    # discovery at <issuer>/.well-known/openid-configuration
clientid = "seanime"
clientsecret = "..."                     # or env SEANIME_OIDC_CLIENT_SECRET
allowedsubjects = ["248289761001"]       # `sub` claims (recommended: stable across renames)
allowedusernames = ["your-username"]     # matched case-insensitively against usernameclaim
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
- Privileged actions — filesystem browsing, process/player launch, extension
  installation, self-update, Nakama host mode — are **denied**, because
  configuring OIDC opts the server out of the local-install default. They are
  granted only by `server.capabilities` (see below), never by a session, a source
  address, or an `Origin` header. A remote authenticated (or compromised) session
  therefore cannot browse the host or run anything on it.
- Secure mode defaults to **`strict`** (was `hardened`), which confines outbound
  fetches (image/stream proxy, extension `fetch`) to publicly routable addresses,
  so a compromised session cannot pivot into your internal network. It no longer
  has any say over privileged actions — that is `capabilities` alone.
- Nakama peers keep authenticating with the Nakama host password header.
- Non-browser clients (e.g. the mobile build) cannot perform the cookie flow;
  point them at a password-mode server instead.

## Capabilities

Anything that reaches past the media library and touches the host is granted in
`config.toml` and nowhere else. The server cannot grant one to itself, and no
request can argue its way into one:

```toml
[server]
capabilities = ["filesystem"]   # "all" / "none" are shorthands
```

| Capability | Grants |
| --- | --- |
| `exec` | spawning processes: media players, torrent clients, a custom `ffmpeg`/`ffprobe`, "open in file explorer", plugin `$os.cmd` |
| `filesystem` | browsing and managing the host filesystem outside the media roots, and repointing the library/download roots |
| `extensions` | installing, updating and reloading third-party extensions — equivalent to code execution, since extension code runs in-process |
| `selfupdate` | downloading a release and replacing the running binary |
| `nakama-host` | serving the peer endpoints, which authenticate with their own shared password instead of the session |

A local install with no reverse proxy, no external URL and no OIDC defaults to
`"all"`; **any** proxy-fronted or OIDC deployment defaults to `"none"`. So on the
topology above you start with everything denied and add back only what you need —
expect a `403` naming the missing capability until you do.

Grant as little as the deployment actually uses. `extensions` and `exec` are each
enough to run arbitrary code as the server user; if you need them, prefer granting
them briefly, installing what you need, and taking them away again. See
[`config.example.toml`](config.example.toml) for the annotated version.

## Hardening checklist

For an internet-facing deployment:

- [ ] **Front it with a TLS-terminating reverse proxy** (Caddy/Traefik/nginx).
      See `docker/Caddyfile` and `docker/compose.yml`.
- [ ] **Do not expose the app port to the host/internet.** Bind loopback on bare
      metal, or in a container keep the port on an internal-only network reachable
      solely by the proxy (the compose example does this).
- [ ] **Keep `securemode = "strict"`** (the OIDC default) so outbound fetches
      cannot reach your internal network.
- [ ] **Leave `capabilities` unset**, and if you must grant one, grant the single
      capability the deployment needs — not `"all"`.
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
      network access, which is why installing one needs the `extensions`
      capability.
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
  Extension installation and other privileged actions stay denied unless you
  grant the matching capability.
- The built-in self-signed TLS (`[server.tls]`) remains available as a fallback,
  but a reverse proxy with real certificates is the recommended setup.

## Stealth gateway (optional)

Sources behind Cloudflare and similar walls can be reached through the cloak-backend
gateway (`seanime-extensions/cloak-backend`), which replays requests with a real Firefox
fingerprint and solves challenges in Camoufox:

```toml
[stealth]
enabled = true
url = "http://127.0.0.1:47541"   # or SEANIME_STEALTH_URL
# token = ""                     # or SEANIME_STEALTH_TOKEN; required when the gateway is not on loopback
extensionMode = "fallback"       # off | fallback | always
officialAPIs = false             # also route AniList, MAL, Jikan, animap, filler, image proxy
```

The gateway refuses a non-loopback bind without `CLOAK_AUTH_TOKEN`. In a compose stack,
put it on Seanime's internal network, set the token on both sides, and do not publish its
port. Datacenter IPs are challenged more than home connections.

## Docker and Podman

A self-contained image and a reverse-proxy-fronted stack are provided. They work with
Podman (rootless or rootful, `podman compose` / `podman-compose`) and with Docker:

- `docker/server.Dockerfile` — builds the web bundle and a static, non-root server image
  with `ffmpeg` for transcoding and a `/api/v1/status` healthcheck.
- `docker/compose.yml` — Seanime on an internal-only network behind Caddy;
  the app port is never published to the host.
- `config.example.toml` — a starting `config.toml` for this topology.
- `docker/Caddyfile` — automatic-HTTPS reverse proxy to the app.

Both Dockerfiles use the repository root as the build context. From the repository root,
with rootless Podman:

```sh
podman build --format docker -t seanime:local -f deploy/docker/server.Dockerfile .   # the image alone

mkdir -p deploy/docker/data
cp deploy/config.example.toml deploy/docker/data/config.toml     # then edit [server.oidc]
podman unshare chown -R 10001:10001 deploy/docker/data           # the container runs as uid 10001
export SEANIME_OIDC_CLIENT_SECRET=...                            # keep the secret out of config.toml
export SEANIME_HTTP_PORT=8080 SEANIME_HTTPS_PORT=8443            # or lower the port floor, below
podman compose -f deploy/docker/compose.yml up -d
```

With Docker or rootful Podman, replace `podman unshare chown` with
`sudo chown -R 10001:10001 deploy/docker/data`, drop the port variables (they default to 80
and 443), and use `docker compose` if you like. Named volumes are chowned automatically;
bind mounts are not.

### Podman notes

- **Data directory ownership.** Rootless Podman maps container uid 10001 to one of your
  subordinate uids, not to host uid 10001, so a plain `sudo chown` leaves the directory
  unwritable. `podman unshare chown` applies the mapping. Afterwards, edit files in it the
  same way: `podman unshare $EDITOR deploy/docker/data/config.toml`.
- **Ports 80/443.** Rootless containers cannot bind ports below 1024. Either publish
  high ports (`SEANIME_HTTP_PORT` / `SEANIME_HTTPS_PORT`) and forward to them at your
  router or firewall, or lower the floor host-wide:
  `echo 'net.ipv4.ip_unprivileged_port_start=80' | sudo tee /etc/sysctl.d/50-unprivileged-ports.conf && sudo sysctl --system`.
  With high ports, Caddy cannot complete ACME's HTTP-01/TLS-ALPN-01 challenges unless
  80/443 reach them from outside; otherwise use a DNS challenge.
- **Client addresses.** Rootless Podman publishes ports through a user-space forwarder,
  which can hide the client's address from Caddy, so everyone arrives as the same IP.
  Seanime's per-client rate limiting and its logs then lose their meaning. Check the
  addresses in Seanime's logs after the first remote login. If they all match, run the
  stack with rootful Podman (`sudo podman compose ...`) or put the host's own reverse
  proxy in front.
- **Starting at boot.** Podman has no daemon, so `restart: unless-stopped` only covers
  crashes. To bring the stack back after a reboot, enable
  `systemctl --user enable --now podman-restart.service` and
  `loginctl enable-linger "$USER"` (rootless), or generate Quadlet units for it.
- **Image healthcheck.** Podman builds OCI images by default, which have no HEALTHCHECK
  field; `--format docker` keeps it. The Compose files declare their own healthcheck,
  so the stack does not depend on this.
- **SELinux** (Fedora, RHEL): the data directory and Caddyfile mounts carry `:z`. The
  media library deliberately does not, since that relabels the whole tree; label it once
  with `chcon -R -t container_file_t /srv/media`.
- Base images are fully qualified (`docker.io/library/...`), so no short-name search
  registry or prompt is involved.

## Serving the UI separately

By default the Go binary serves the web UI itself. To serve the bundle from your own web
server instead, run Seanime API-only and let the web server serve `seanime-web/out`:

- Build with the `noembedweb` tag (`docker/server.Dockerfile --build-arg EMBED_WEB=false`). The
  server logs "running in API-only mode" and answers non-API page requests with 404.
- UI and API **must share one origin**. The OIDC session cookie is `__Host-` scoped, and
  state-changing requests from another origin are rejected.
- Proxy these paths to Seanime: `/api/*`, `/events` (websocket), `/assets/*`,
  `/manga-downloads/*`, `/offline-assets/*`. Serve everything else from the bundle, with
  `index.html` as the fallback for unknown paths.
- With OIDC on, gate the bundle with a forward-auth subrequest to
  `GET /api/v1/auth/session-check`. It returns 204 with a valid session, a redirect to
  `/login` for page loads without one, and 401 otherwise; without OIDC it always returns 204.
  Serve `/login` (as `shell/index.html`) and `/shell/*` from the bundle without the gate.
- Set `Cross-Origin-Opener-Policy: same-origin` and
  `Cross-Origin-Embedder-Policy: credentialless` on UI responses. The subtitle renderer needs
  `SharedArrayBuffer`, which browsers only allow on cross-origin-isolated pages.
- `server.trustedProxies` must include the web server, as for any reverse proxy.

Files for this setup:

- `docker/web.Dockerfile` — Caddy with the built bundle at `/srv` and the Caddyfile below.
- `docker/Caddyfile.decoupled` — the routing, gate and headers above.
- `docker/compose.decoupled.yml` — override for `docker/compose.yml`:

```sh
podman compose -f deploy/docker/compose.yml -f deploy/docker/compose.decoupled.yml up -d   # or docker compose
```

## systemd (bare metal)

`sudo packaging/linux/install.sh --system` is the non-container equivalent: a dedicated
`seanime` user and a unit sandboxed like the pod in `k8s/deployment.yaml`. It seeds
`capabilities = []` into a new config; configure the reverse proxy, `externalurl`,
`trustedproxies` and `[server.oidc]` before exposing it. Unit trade-offs (`selfupdate`,
`ReadWritePaths=` per media root) are in
[`packaging/linux/README.md`](../packaging/linux/README.md#system-wide---system).
