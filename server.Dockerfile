# Self-contained, production-oriented image for a server-side Seanime deployment.
# Builds the web bundle and the Go binary from source, then ships a slim,
# non-root runtime. Intended to run behind a TLS-terminating reverse proxy.
#
#   docker build -t seanime:local -f server.Dockerfile .
#
# See docker-compose.example.yml for the recommended reverse-proxy topology.

# ---- Stage 1: build the web bundle (embedded into the Go binary) ----------
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

# ---- Stage 2: build the static Go server ----------------------------------
FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal ./internal
COPY codegen ./codegen
COPY test ./test
COPY docs ./docs

# The web bundle is embedded via //go:embed all:web in main.go.
COPY --from=web /src/seanime-web/out ./web

# Matches the CI server build: fully static (pure-Go sqlite + D-Bus), no CGO,
# systray compiled out for a headless server.
RUN CGO_ENABLED=0 go build -tags=nosystray -trimpath -ldflags="-s -w" -o /out/seanime .

# ---- Stage 3: runtime -----------------------------------------------------
FROM debian:12-slim AS runtime

# ffmpeg/ffprobe are required for on-the-fly transcoding; curl is used only by
# the container healthcheck; ca-certificates for outbound TLS (OIDC discovery,
# metadata providers).
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

# Non-root runtime user (best practice; container never runs as root).
RUN useradd --create-home --uid 10001 appuser \
    && mkdir -p /data \
    && chown -R appuser:appuser /data

COPY --from=build /out/seanime /usr/local/bin/seanime

USER appuser

# All mutable state (db, logs, cache, extensions, assets) lives under the data
# dir. Bind or volume-mount this in production.
ENV SEANIME_DATA_DIR=/data
# Bind inside the container only. Do NOT publish this port to the host; let the
# reverse proxy reach it over an internal Docker network.
ENV SEANIME_SERVER_HOST=0.0.0.0
ENV SEANIME_SERVER_PORT=43211

VOLUME ["/data"]
EXPOSE 43211

# /api/v1/status is a public endpoint (no session required), so it is a safe
# liveness probe.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fsS "http://127.0.0.1:43211/api/v1/status" || exit 1

ENTRYPOINT ["/usr/local/bin/seanime"]
