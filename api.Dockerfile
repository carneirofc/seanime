FROM golang:1.25


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

RUN go build -v

# Run as a non-root user to avoid a container running as root (best practice; DS-0002).
RUN useradd --create-home --uid 10001 appuser \
    && chown -R appuser /usr/src/app
USER appuser