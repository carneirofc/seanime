# Seanime Development and Build Guide

Stack: Go (Echo, GORM/SQLite, Goja plugins) and a React + Rsbuild + TanStack Router web UI.
`hls.js` is pinned to `1.5.20`: 1.6.0 and above cause `appendBuffer` fatal errors.

## Quick start

Needs Go 1.27+ (`go.mod` sets `1.27`; CI builds on 1.27.2) and Node.js 20+ with npm.
Everything runs from the repository root; **`npm run` is the interface** on every OS.

```bash
npm ci            # once, and again whenever package-lock.json changes
npm run dev       # develop: Go server :43000 + web UI :43210, both reloading
npm run build     # release build: dist/seanime, web UI embedded
```

Open <http://127.0.0.1:43210> while `npm run dev` runs. `npm run` with no arguments lists
every script.

## Where things are

| Path | What |
| --- | --- |
| `main.go`, `internal/` | The Go server ([`internal/AGENTS.md`](internal/AGENTS.md)) |
| `seanime-web/` | The web UI ([`seanime-web/AGENTS.md`](seanime-web/AGENTS.md)) |
| `codegen/` | Go → TypeScript contract generator ([`codegen/README.md`](codegen/README.md)) |
| `web/` | Build output: the UI copied here to be embedded in the binary. Do not edit |
| `dist/` | Build output: binaries and installers |
| `dev-datadir/` | Data directory of the dev server (gitignored) |
| `scripts/` | Arch Linux automation wrappers around the npm scripts ([Arch Linux](#arch-linux)) |
| `deploy/` | Running it as a server: config example, Docker, Compose, Caddy, Kubernetes ([`deploy/README.md`](deploy/README.md)) |
| `packaging/` | Installing it on a machine: Linux and Windows installers, Arch package, AppImage ([`packaging/README.md`](packaging/README.md)) |
| `test/` | Test config template and AniList fixtures ([Testing](#testing)) |
| `patches/` | `patch-package` patches, applied by the root `postinstall` |

## Commands

All from the repository root.

| Goal | Command | Result |
| --- | --- | --- |
| Install dependencies | `npm ci` | npm workspace (root + `seanime-web`), patches applied |
| Develop everything | `npm run dev` | codegen watcher, Go server `:43000`, web UI `:43210` |
| Develop one part | `npm run dev:go` · `dev:web` · `dev:codegen` | that process alone |
| Regenerate the API contract | `npm run codegen` | same as `go generate ./codegen` |
| Build the release binary | `npm run build` | `dist/seanime` (`dist\seanime-windows-amd64.exe`) |
| Build the web UI only | `npm run build:web` | `seanime-web/out/` |
| Install on this machine | `npm run build:install` | Linux: XDG entry + icons; Windows: shortcuts |
| Windows `setup.exe` | `npm run build:all` | `dist\seanime-setup-<version>.exe` (needs Inno Setup) |
| Container image | `podman build --format docker -f deploy/docker/server.Dockerfile .` (or `docker build`) | see [`deploy/README.md`](deploy/README.md#docker-and-podman) |
| Web checks | `npm run typecheck` · `test:web` · `lint:web` | tsgo, Vitest, Biome on `seanime-web` |
| Go tests | `go test ./internal/<pkg>/...` | `npm run test:go` runs everything, including packages that need local fixtures; see [Testing](#testing) |

Ports: the dev server listens on `43000` because the dev web client sends its localhost API
calls there (`seanime-web/src/lib/server/config.ts`). A built binary defaults to `43211`.

## Build

`npm run build` runs these steps in order, each also available on its own:

| Step | Script | What it does |
| --- | --- | --- |
| 1 | `npm run clean` | Empties `web/` (keeping the tracked `web/.gitkeep`), removes `dist/` and `seanime-web/out`. |
| 2 | `npm run codegen` | Regenerates the Go → TypeScript contract (`go generate ./codegen`). |
| 3 | `npm run build:web` | Typechecks and builds the web interface into `seanime-web/out`. |
| 4 | `npm run build:embed` | Copies that output into the root `web/` directory. |
| 5 | `npm run build:api` | Builds the server binary into `dist/`. |

The order matters: `webfs_embed.go` embeds the web interface with `//go:embed all:web`, so
step 5 fails with `pattern all:web: no matching files found` if `web/` is missing or empty.
`web/.gitkeep` is tracked so a clean checkout always compiles.

Step 5 is the headless, fully static build used by CI and both Dockerfiles:

```bash
CGO_ENABLED=0 go build -tags=nosystray -trimpath -ldflags="-s -w" -o dist/seanime .
```

Add the `noembedweb` tag for an API-only binary that carries no web UI and needs no `web/`
directory. It is for deployments where a separate web server serves the bundle on the same
origin (see [`deploy/README.md`](deploy/README.md), "Serving the UI separately"):

```bash
CGO_ENABLED=0 go build -tags=nosystray,noembedweb -trimpath -ldflags="-s -w" -o dist/seanime .
```

The Windows system-tray variant needs CGO and a mingw toolchain and is only built in CI:

```bash
CGO_ENABLED=1 GOOS=windows go build -o seanime.exe -trimpath \
    -ldflags="-s -w -H=windowsgui -extldflags '-static'"
```

## Development

### The whole stack, one terminal

`npm run dev` starts three processes:

- a codegen watcher that reruns `go generate ./codegen` when Go/codegen sources change
  (it ignores generated output, so it does not loop),
- the Go server from the repository root on `127.0.0.1:43000`, using the gitignored
  `dev-datadir/` as its data directory, reloading when Go sources change,
- the `seanime-web` dev server on `127.0.0.1:43210`.

`npm run dev:codegen`, `dev:go` and `dev:web` run each process alone.

### Server only

```bash
go run . --datadir=path/to/datadir
```

The data directory is created and populated on first run. The path may be relative; it is
resolved against the directory you run the command from. Omit `--datadir` and the OS config
directory is used. `SEANIME_DATA_DIR` does the same job as the flag.

`//go:embed all:web` is satisfied by the tracked `web/.gitkeep`, so the server always
compiles; it serves a real UI only after `npm run build:web && npm run build:embed`. Or skip
the UI with `go run -tags=noembedweb .` and use the web dev server.

To reach the server from other devices, set `host` to `0.0.0.0` in the `config.toml` inside
the data directory.

### Web only

```bash
npm run dev:web
```

Serves on `127.0.0.1:43210` and expects a Go server on `43000`.
`npm run dev:mobile --workspace seanime-web` builds against `.env.mobile` instead of `.env.web`.

## Install and package

Build and install in one step, or install an already-built binary:

```bash
npm run build:install     # build, then run the installer for this platform
npm run install:linux     # packaging/linux/install.sh: XDG entry, icons, optional systemd unit
npm run install:windows   # packaging/windows/install.ps1: shortcuts and a user PATH entry
```

Both run their installer with no options. Flags cannot travel through `npm run`, so call the
script directly to pass any — `packaging/linux/install.sh --help`, or
`powershell -File .\packaging\windows\install.ps1`.

Every installer and package format is listed in [`packaging/README.md`](packaging/README.md).
Running it as a server (Docker, Kubernetes, reverse proxy, OIDC) is in
[`deploy/README.md`](deploy/README.md).

## Arch Linux

`scripts/` holds the same five commands as the author's other Arch-packaged projects.
They wrap the npm scripts above, run from any directory, and suit automation such as
Ansible: they never prompt without a terminal, log on stderr and print only result paths
on stdout, and exit 3 when pacman packages are missing.

```bash
sudo pacman -S --needed $(scripts/setup.sh --deps)
scripts/setup.sh            # npm ci and the Go modules
scripts/dev.sh              # npm run dev (scripts/dev.sh go|web|codegen for one part)
scripts/build.sh            # npm run build; prints dist/seanime
scripts/install.sh          # build, then packaging/linux/install.sh for this user (its flags pass through)
scripts/package.sh -si      # packaging/arch/PKGBUILD from the committed HEAD, installed
```

`scripts/package.sh` builds the split package in `~/.cache/arch-build/seanime`, leaving
the checkout clean: `seanime-git` (the server, launcher, desktop entry, icons and a
systemd user unit, `systemctl --user enable --now seanime`) and
`seanime-cloak-backend-git` (the stealth gateway); see
[`packaging/arch/AGENTS.md`](packaging/arch/AGENTS.md). `scripts/lib.sh` is shared
verbatim with those other repositories; `scripts/project.sh` holds what is seanime's.

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
