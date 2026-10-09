# Seanime Development and Build Guide

Stack: Go (Echo, GORM/SQLite, Goja plugins) and a React + Rsbuild + TanStack Router web UI.
`hls.js` is pinned to `1.5.20`: 1.6.0 and above cause `appendBuffer` fatal errors.

## Prerequisites

- Go 1.27+ (`go.mod` sets `1.27`; CI builds on 1.27.1)
- Node.js 20+ and npm

## Build

From the repository root, one command builds everything on Linux, macOS and Windows:

```bash
npm run build
```

It runs these steps in order, each also available on its own:

| Step | Script | What it does |
| --- | --- | --- |
| 1 | `npm run clean` | Removes `web/`, `dist/` and `seanime-web/out`. |
| 2 | `npm run codegen` | Regenerates the Go → TypeScript contract (`go generate ./codegen`). |
| 3 | `npm run build:web` | Typechecks and builds the web interface into `seanime-web/out`. |
| 4 | `npm run build:embed` | Copies that output into the root `web/` directory. |
| 5 | `npm run build:api` | Builds the server binary into `dist/`. |

`npm run build:all` also packages the Windows installer (see below).

The order matters: `webfs_embed.go` embeds the web interface with `//go:embed all:web`, so
step 5 fails with `pattern all:web: no matching files found` if `web/` is missing or empty.

The binary lands at `dist/seanime` (`dist\seanime-windows-amd64.exe` on Windows). Step 5 is
the headless, fully static build used by CI and both Dockerfiles:

```bash
CGO_ENABLED=0 go build -tags=nosystray -trimpath -ldflags="-s -w" -o dist/seanime .
```

Add the `noembedweb` tag for an API-only binary that carries no web UI and needs no `web/`
directory. It is for deployments where a separate web server serves the bundle on the same
origin (see `WEB_DEPLOYMENT.md`, "Serving the UI separately"):

```bash
CGO_ENABLED=0 go build -tags=nosystray,noembedweb -trimpath -ldflags="-s -w" -o dist/seanime .
```

The Windows system-tray variant needs CGO and a mingw toolchain and is only built in CI:

```bash
CGO_ENABLED=1 GOOS=windows go build -o seanime.exe -trimpath \
    -ldflags="-s -w -H=windowsgui -extldflags '-static'"
```

## Install

Build and install in one step, or install an already-built binary:

```bash
npm run build:install     # build, then run the installer for this platform
npm run install:linux     # installs an XDG desktop entry, icons, optional systemd unit
npm run install:windows   # installs shortcuts and a user PATH entry
```

Both run their installer with no options. Flags cannot travel through `npm run`, so call the
script directly to pass any — `./install-linux.sh --help`, or
`powershell -File .\install-windows.ps1`.

- Linux: [`installer/linux/README.md`](installer/linux/README.md) — full flag list, what lands
  where, and the difference between the two systemd units.
- Windows: [`installer/README.md`](installer/README.md) — the script, plus the Inno Setup
  `setup.exe` packaging.
- Exposing a server to the internet: [`WEB_DEPLOYMENT.md`](WEB_DEPLOYMENT.md).

## Development

### The whole stack, one terminal

```bash
npm install
npm run dev
```

That starts three processes:

- a codegen watcher that reruns `go generate ./codegen` when Go/codegen sources change
  (it ignores generated output, so it does not loop),
- the Go server from the repository root on `127.0.0.1:43000`, using the gitignored
  `dev-datadir/` as its data directory, reloading when Go sources change,
- the `seanime-web` dev server on `127.0.0.1:43210`.

The web client routes its localhost requests to port `43000`
(`seanime-web/src/lib/server/config.ts`), which is why the Go server uses it in development.

`npm run dev:codegen`, `dev:go` and `dev:web` run each process alone.

### Server only

```bash
go run . --datadir=path/to/datadir
```

The data directory is created and populated on first run. The path may be relative; it is
resolved against the directory you run the command from. Omit `--datadir` and the OS config
directory is used. `SEANIME_DATA_DIR` does the same job as the flag.

The server needs a `web/` directory at the root to satisfy `//go:embed all:web`. Either run
`npm run build:web && npm run build:embed`, create `web/` with any one file in it, or skip
the UI with `go run -tags=noembedweb .` and use the Rsbuild dev server (`npm run dev` in
`seanime-web/`).

To reach the server from other devices, set `host` to `0.0.0.0` in the `config.toml` inside
the data directory.

### Web only

```bash
cd seanime-web
npm install
npm run dev
```

Serves on `127.0.0.1:43210` and expects a Go server on `43000`.

## Codebase architecture

### Routes and handlers

- Routes are registered in `internal/handlers/routes.go`, each bound to a handler method.
- Handlers live in `internal/handlers/` and carry doc comments above the declaration
  (`@summary`, `@route`, `@param`, `@returns`) that double as the input to codegen.

### Codegen

The doc comments and exported structs generate the frontend's TypeScript contract. After
changing a handler, a returned struct, or a plugin event, regenerate from the repository root:

```bash
go generate ./codegen
```

Never `go run ./codegen`. CI fails if the committed output is stale. See
[`codegen/README.md`](codegen/README.md) for what the generator writes, the annotation grammar,
and its known constraints.

### AniList GraphQL

- Queries live in `internal/api/anilist/queries/*.graphql`; `gqlgenc` turns them into
  `internal/api/anilist/client_gen.go` and `models_gen.go`, configured by
  `internal/api/anilist/.gqlgenc.yml`.
- `internal/api/anilist/client.go` wraps the generated client with a cleaner interface;
  `client_mock.go` is the fixture-backed mock used by tests.

To regenerate after a schema or query change:

```bash
cd internal/api/anilist && go run github.com/gqlgo/gqlgenc && cd ../../.. && go mod tidy
```

## Testing

```bash
go test ./path/to/package/...
```

`go test ./...` does **not** pass on a clean checkout — some packages need gitignored fixtures
or a local config, and a few fail on `main` for their own reasons. CI runs an explicit subset.
See the Testing section of [`internal/AGENTS.md`](internal/AGENTS.md) for the three causes, the
excluded-package list, and how to run the excluded ones locally.

### Test configuration

Copy `test/config.example.toml` to `test/config.toml` and fill in what the tests you are
running need:

- An AniList access token for a throwaway account, for the AniList-backed packages.
- Connection details for Transmission/qBittorrent, for the torrent-client tests.
- `path.sampleVideoPath` (or `TEST_SAMPLE_VIDEO_PATH` in the environment) for media-player
  tests that open a file.

AniList fixtures under `test/testdata/` are read-only during normal runs; a missing one fails
the test rather than hitting the network. Set `SEANIME_TEST_RECORD_ANILIST_FIXTURES=true` to
record or refresh them against a live, authenticated client:

```bash
SEANIME_TEST_RECORD_ANILIST_FIXTURES=true go test ./internal/api/anilist/...
```

### Writing tests

Use the shared helpers in `internal/testutil` (`InitTestProvider`, `NewTestEnv`, fixture
helpers, `RequireSampleVideoPath`) and the mock builders in `internal/testmocks`; see
[`CONTRIBUTING.md`](CONTRIBUTING.md#testing).
