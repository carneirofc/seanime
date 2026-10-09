# The Seanime web UI served by Caddy, for deployments where the Go server runs
# API-only (server.Dockerfile --build-arg EMBED_WEB=false). Caddy terminates TLS,
# serves the bundle and proxies the API on the same origin; see
# example.decoupled.Caddyfile and docker-compose.decoupled.example.yml.
#
#   docker build -t seanime-web:local -f web.Dockerfile .

# ---- Stage 1: build the web bundle ----------------------------------------
FROM node:22-slim AS web
WORKDIR /src

# Install workspace dependencies first for better layer caching. patches/ is
# required because the root postinstall runs patch-package.
COPY package.json package-lock.json ./
COPY patches ./patches
COPY seanime-web/package.json seanime-web/package-lock.json ./seanime-web/
RUN npm ci

# Build the static web output (rsbuild -> seanime-web/out).
COPY seanime-web ./seanime-web
RUN npm run build:web

# ---- Stage 2: Caddy with the bundle ---------------------------------------
FROM caddy:2-alpine

COPY --from=web /src/seanime-web/out /srv
COPY example.decoupled.Caddyfile /etc/caddy/Caddyfile
