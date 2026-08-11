# Builds the Go server against a web bundle provided by a pre-built
# `seanime-web:latest` image (the existing two-image flow). For a self-contained
# single-command build, use the top-level `Dockerfile` instead.

FROM golang:1.26

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY codegen ./codegen
COPY test ./test
COPY internal ./internal
COPY docs ./docs

COPY --from=seanime-web:latest /app/web /usr/src/app/web

# Fully static, headless server build (matches CI): pure-Go sqlite, no CGO,
# systray compiled out.
RUN CGO_ENABLED=0 go build -tags=nosystray -trimpath -ldflags="-s -w" -o /usr/local/bin/seanime .

# Run as a non-root user to avoid a container running as root (best practice; DS-0002).
RUN useradd --create-home --uid 10001 appuser \
    && mkdir -p /data \
    && chown -R appuser:appuser /data /usr/src/app
USER appuser

ENV SEANIME_DATA_DIR=/data
# Container-internal bind only; do not publish to the host (reverse proxy fronts it).
ENV SEANIME_SERVER_HOST=0.0.0.0
ENV SEANIME_SERVER_PORT=43211

VOLUME ["/data"]
EXPOSE 43211

ENTRYPOINT ["/usr/local/bin/seanime"]
