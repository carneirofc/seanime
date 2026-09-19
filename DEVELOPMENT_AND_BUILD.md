# Seanime Development and Build Guide

## Tech stack

* Server: Built with [Go](https://go.dev/)
    * API: [Echo](https://echo.labstack.com/)
    * Plugin system: [Goja](https://github.com/dop251/goja) for Javascript runtimes
    * Database: [SQLite](https://github.com/glebarez/sqlite) handled via [GORM](https://gorm.io/)
    * File scanner: [Habari](https://github.com/5rahim/habari) for filename parsing
    * Torrent streaming: [anacrolix/torrent](https://github.com/anacrolix/torrent) for Bittorrent client
    * OS Integration: [Fyne](https://github.com/fyne-io/systray) for Windows system tray management
    * MKV Parser: Fork of [matroska-go](https://github.com/luispater/matroska-go)
* Frontend: Built with [React](https://reactjs.org/), [Rsbuild](https://rsbuild.dev/), and [Tanstack Router](https://tanstack.com/router)
	* UI Library: Custom components built with [Tailwind](https://tailwindcss.com/) and [Radix UI](https://www.radix-ui.com/)
	* Data Fetching: [React Query](https://tanstack.com/query/latest)
	* State Management: [Jotai](https://jotai.org/) for global state
	* Built-in Player: Custom-made (VideoCore)

## Prerequisites

- Go 1.23+
- Node.js 18+ and npm

## Build Process

From the repository root, one command builds everything on Linux, macOS and Windows:

```bash
npm run build
```

It runs three steps in order, each also available on its own:

| Step | Script | What it does |
| --- | --- | --- |
| 1 | `npm run build:web` | Typechecks and builds the web interface into `seanime-web/out`. |
| 2 | `npm run build:embed` | Replaces the root `web/` directory with that output. |
| 3 | `npm run build:api` | Builds the server binary into `dist/`. |

The order matters: `main.go` embeds the web interface with `//go:embed all:web`, so step 3
fails with `pattern all:web: no matching files found` if `web/` is missing or empty.

The binary lands at `dist/seanime` (`dist\seanime-windows-amd64.exe` on Windows). Step 3 is
the headless, fully static build used by CI and both Dockerfiles:

```bash
CGO_ENABLED=0 go build -tags=nosystray -trimpath -ldflags="-s -w" -o dist/seanime .
```

The Windows system-tray variant needs CGO and a mingw toolchain and is only built in CI:

```bash
CGO_ENABLED=1 GOOS=windows go build -o seanime.exe -trimpath \
    -ldflags="-s -w -H=windowsgui -extldflags '-static'"
```

### Installing on Windows

After building, you can install the server for the current user with the bundled
installer script. It copies the binary into an install directory, creates the data
directory, and optionally adds a Start Menu/Desktop shortcut and a PATH entry.

Build and install in one step:

```bash
npm run build:install
```

Or install an already-built binary:

```bash
npm run install:windows
```

The installer (`install-windows.ps1`) accepts options when invoked directly, e.g.:

```powershell
# Build first, install to a custom directory, add to PATH and create a desktop shortcut
powershell -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 `
    -BuildFirst -InstallDir "D:\Apps\Seanime" -AddToPath -DesktopShortcut
```

By default it installs to `%LOCALAPPDATA%\Programs\Seanime`, uses
`<InstallDir>\seanime_data_dir` as the data directory, and looks for the binary
produced by `npm run build` at `dist\seanime-windows-amd64.exe`.

---

## Development Guide

### Getting Started

The project is built with:
- Backend: Go server with REST API endpoints
- Frontend: React + Vite + Tanstack Router

For development, you should be familiar with both Go and React.

### Setting Up the Development Environment

#### Server Development

1. **Development environment**:
   - Create a dummy directory that will be used as the data directory during development.
   - Create a dummy `web` folder at the root containing at least one file, or simply do the _Building the Web Interface_ step of the build process. (This is required for the server to start.)

2. **Run the server**:
    ```bash
    go run main.go --datadir="path/to/datadir"
    ```
   
	- This will generate all the files needed in the `path/to/datadir` directory.
	- The path may be relative; it is resolved against the directory you run the command from.
	  Omit `--datadir` entirely and the OS config directory is used.
   
3. **Configure the development server**:
   - Change the port in the `config.toml` located in the development data directory to `43001`. The web interface will connect to this port during development. Change the host to `0.0.0.0` to allow connections from other devices.
   - Re-run the server with the updated configuration.

   The server will be available at `http://127.0.0.1:43001`.

#### Combined root workspace development

If you want to run both the Go server and the web dev server from one terminal, use the repo root workspace:

1. **Install workspace dependencies**:
   ```bash
   npm install
   ```

2. **Start the root development stack**:
   ```bash
   npm run dev
   ```

    This starts:
    - a watched codegen process for Go handler/struct/plugin event changes
    - a watched Go server process from the repository root on `127.0.0.1:43001`, using
      the gitignored `dev-datadir/` as its data directory
    - the `seanime-web` dev server

    `dev:go` passes `--datadir=$INIT_CWD/dev-datadir`. The server rejects a relative data
    directory, and `$INIT_CWD` is how the absolute path stays portable: on Unix the shell
    expands it, and on Windows `cmd.exe` leaves it alone so the server's own
    `os.ExpandEnv` resolves it (`internal/core/config.go`). `mprocs.yaml` is an alternative
    to `concurrently` and runs the same two scripts.

    The combined command keeps the full development stack in the same terminal and reloads the Go server when Go source files change. The root workspace uses port `43001` for the Go server so it matches the web client's localhost development routing.

3. **Run only codegen watch if needed**:
   ```bash
   npm run dev:codegen
   ```

   The codegen watcher reruns `go generate ./codegen` when relevant Go/codegen sources change and ignores generated output files to avoid watch loops.

#### Web Interface Development

1. **Navigate to the web directory**:
   ```bash
   cd seanime-web
   ```

2. **Install dependencies**:
   ```bash
   npm install
   ```

3. **Start the development server**:
   ```bash
   npm run dev
   ```

   The development web interface will be accessible at `http://127.0.0.1:43210`.

**Note**: During development, the web interface is served by the Rsbuild dev server on port `43210`.
It is configured such that localhost requests are made to the Go server running on port `43001`.

### Understanding the Codebase Architecture

#### API and Route Handlers

The backend follows a well-defined structure:

1. **Routes Declaration**: 
   - All routes are registered in `internal/handlers/routes.go`
   - Each route is associated with a specific handler method

2. **Handler Implementation**:
   - Handler methods are defined in `internal/handlers/` directory
   - Handlers are documented with comments above each declaration (similar to OpenAPI)

3. **Automated Type Generation**:
   - The comments above route handlers serve as documentation for automatic type generation
   - Types for the frontend are generated in:
     - `seanime-web/api/generated/types.ts`
     - `seanime-web/api/generated/endpoint.types.ts`
     - `seanime-web/api/generated/hooks_template.ts`

#### Updating API Types

After modifying route handlers or structs used by the frontend, you must regenerate the TypeScript types:

```bash
# Run the code generator
go generate ./codegen/main.go
```

#### AniList GraphQL API Integration

The project integrates with the AniList GraphQL API:

1. **GraphQL Queries**:
   - Queries are defined in `internal/anilist/queries/*.graphql`
   - Generated using `gqlgenc`

2. **Updating GraphQL Schema**:
   If you modify the GraphQL schema, run these commands:

```bash
go get github.com/gqlgo/gqlgenc@v0.33.1
```
```bash
cd internal/api/anilist
```
```bash
go run github.com/gqlgo/gqlgenc
```
```bash
cd ../../..
```
```bash
go mod tidy
```

3. **Client Implementation**:
   - Generated queries and types are in `internal/api/anilist/client_gen.go`
   - A wrapper implementation in `internal/api/anilist/client.go` provides a cleaner interface
   - The wrapper also includes a mock client for testing

### Running Tests

**Important**: Run tests individually rather than all at once.

#### Test Configuration

1. Create a dummy AniList account for testing
2. Obtain an access token (from browser)
3. Create/edit `test/config.toml` using `config.example.toml` as a template

#### Writing Tests

Tests use the `internal/testutil` package which provides:

- `InitTestProvider` to load test configuration and apply feature-flag skips
- `NewTestEnv` to create an isolated temp root, app data dir, cache dir, and database for tests
- `FixtureRelPath` and fixture helpers
- `RequireSampleVideoPath` for media-player tests that need a real sample file

Example:
```go
func TestSomething(t *testing.T) {
       env := testutil.NewTestEnv(t, testutil.Anilist())
       database := env.MustNewDatabase(util.NewLogger())
       _ = database
}
```

AniList mock fixtures are read-only during normal test runs. Set `SEANIME_TEST_RECORD_ANILIST_FIXTURES=true` when you intentionally want missing or refreshed fixtures written back to the repository.

To avoid remembering the environment variable and basic auth checks, use the refresh wrapper:

```bash
go run ./scripts/record_anilist_fixtures
```

Notes:

- It validates that `test/config.toml` exists, `flags.enable_anilist_tests=true`, and `provider.anilist_jwt` is set.
- It defaults to refreshing `./internal/api/anilist` and sets `SEANIME_TEST_RECORD_ANILIST_FIXTURES=true` for the test process.
- Pass packages to widen the refresh scope, for example `go run ./scripts/record_anilist_fixtures ./internal/api/anilist ./internal/library/scanner`.
- Pass `-run` to target specific live refresh tests, for example `go run ./scripts/record_anilist_fixtures -run 'TestGetAnimeByIdLive|TestBaseAnime_FetchMediaTree_BaseAnimeLive'`.

#### Testing with Third-Party Apps

Some tests interact with applications like Transmission and qBittorrent:
- Ensure these applications are installed and running
- Configure `test/config.toml` with appropriate connection details

Media-player tests that open a file also require `path.sampleVideoPath` in `test/config.toml`, or `TEST_SAMPLE_VIDEO_PATH` in the environment.

## Notes and Warnings

- hls.js versions 1.6.0 and above may cause appendBuffer fatal errors
