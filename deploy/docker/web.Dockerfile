# The Seanime web UI served by Caddy, for deployments where the Go server runs
# API-only (server.Dockerfile --build-arg EMBED_WEB=false). Caddy terminates TLS,
# serves the bundle and proxies the API on the same origin; see
# Caddyfile.decoupled and compose.decoupled.yml next to this file.
#
#   podman build -t seanime-web:local -f deploy/docker/web.Dockerfile .   # from the repo root
#   docker build -t seanime-web:local -f deploy/docker/web.Dockerfile .
#
# Base images are fully qualified so Podman needs no short-name search registry.

# ---- Stage 1: build the web bundle ----------------------------------------
FROM docker.io/library/node:22-slim AS web
WORKDIR /src

# Install workspace dependencies first for better layer caching. patches/ is
# required because pnpm-workspace.yaml patches dependencies on install. pnpm
# itself is installed at the version package.json pins in packageManager.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY patches ./patches
COPY seanime-web/package.json ./seanime-web/
RUN npm install -g "$(node -p 'require("./package.json").packageManager')" \
    && pnpm install --frozen-lockfile

# Build the static web output (rsbuild -> seanime-web/out).
COPY seanime-web ./seanime-web
RUN pnpm run build:web

# ---- Stage 2: Caddy with the bundle ---------------------------------------
FROM docker.io/library/caddy:2-alpine

# Non-root runtime user, the same uid as the Seanime image. The caddy binary
# carries cap_net_bind_service, so it still binds :80 and :443. Its only
# writable state is /data (certificates) and /config (autosaved config); owning
# them here means a fresh named volume mounted there inherits the ownership.
RUN addgroup -S -g 10001 caddy \
    && adduser -S -D -H -u 10001 -G caddy -s /sbin/nologin caddy \
    && chown -R caddy:caddy /data /config

COPY --from=web /src/seanime-web/out /srv
COPY deploy/docker/Caddyfile.decoupled /etc/caddy/Caddyfile

USER caddy
