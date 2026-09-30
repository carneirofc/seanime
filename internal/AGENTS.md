# Seanime Backend Agents

Go backend powering the Seanime API and serving the embedded web UI. Covers runtime slices,
entrypoints, and how backend changes flow to the frontend.

## At-a-glance

| Agent | Scope | Key Paths | Runtime | Typical Commands |
| --- | --- | --- | --- | --- |
| Core Server | Boot, flags, config, logging, updater | `main.go`, `internal/server/`, `internal/core/` | Go 1.26.5 | `go run .`, `go build -o seanime` |
| HTTP API + Events | REST endpoints and websocket events | `internal/handlers/`, `internal/core/echo.go` | Echo v4 | `go test ./internal/...` |
| Embedded Web UI | Serve the built web UI | `web/`, `internal/core/echo.go` | Go embed FS | `npm run build` at repo root |
| Background Jobs | Recurring sync/update loops | `internal/cron/` | Go | Runs with server startup |
| Codegen | Emit frontend types from Go | `codegen/` | Go | `go generate ./codegen` |

## Core Server

- `main.go` embeds `web/` and `internal/icon/logo.png`, then calls `internal/server.StartServer`.
- OS-specific server entrypoints in `internal/server/server_{unix,windows}.go`.
- Flags parsed in `internal/core/app.go`; config and logging in `internal/core/config.go` and `internal/util/`.

### Updater source

- `internal/updater` checks `api.github.com/repos/carneirofc/seanime/releases/latest` only; the
  upstream `seanime.app` channels are not used, and a stored `updateChannel` is ignored.
- `util.ValidateReleaseUrl` accepts only `carneirofc` GitHub release assets.
- `constants.AnnouncementURL` is empty (no fork feed yet), which disables announcements.

## Extensions

- Runtime health: every Goja provider call goes through `gojaProviderBase.callClassMethod`,
  which records its outcome in `extension_repo.HealthTracker`. After `failingThreshold`
  consecutive failures an extension is failing: it is reported in `AllExtensions.health`,
  an `extension-failing` event is sent, and it is disabled if
  `StoredExtensionSettingsData.AutoDisableFailing` is on. Health is in memory and resets on reload.
- Plugins are not covered: they have no single call path.
- Provider methods must return the error from `callClassMethod`; discarding it makes a failing
  source look like one that returned no results.

## AniList entry privacy

- Adult media first added to the list defaults to `private` + `hiddenFromStatusLists`
  (setting on by default). It is applied server-side on the add path
  (`HandleEditAnilistListEntry`, `AddMediaToCollection`) and keyed on nil-vs-explicit flag
  pointers, so an explicit user choice always wins. Later saves never re-force the flags.
- Instead of enforcing, the UI alerts when an adult entry is public. Exposure is keyed on
  `private` only, and the alert fires regardless of the setting.
- `Platform.UpdateEntry` takes an `UpdateEntryParams` struct; its narrow siblings
  `UpdateEntryProgress` / `UpdateEntryRepeat` keep positional arguments on purpose.

## Manga downloads

- Downloaded chapters are CBZ archives with a `ComicInfo.xml`
  (`internal/manga/downloader/`); the struct's field order is the ComicInfo XSD sequence, so
  new fields must be inserted in schema order.
- ComicInfo metadata is resolved once per download batch by
  `manga.ResolveDownloadSeriesMetadata` and persisted as JSON in the queue item's `metadata`
  column. Every lookup is best-effort and must never fail a download.
- Staff (authors/artists) use a custom AniList query in `internal/manga/download_metadata.go`
  instead of the generated client, to avoid regenerating it; it is skipped for custom-source ids.

## HTTP API + Events

- Echo instantiated in `internal/core/echo.go` with JSON serialization overrides.
- Routes registered in `internal/handlers`, mounted under `/api`.
- Event streams served under `/events` (websocket/SSE handlers in `internal/handlers`).

## Embedded Web UI

- Static web UI embedded via `//go:embed all:web` in `main.go`.
- `internal/core/echo.go` mounts the embedded `web/` filesystem with HTML5 fallback routing.
- Static mounts from config: `/assets` → `app.Config.Web.AssetDir`,
  `/manga-downloads` → `app.Config.Manga.DownloadDir`, `/offline-assets` → `app.Config.Offline.AssetDir`.
- Don't edit `web/` by hand; it is a build artifact from `seanime-web/`, written by `npm run build` (or `build:embed`) at the repo root.

## Backend → Frontend Codegen

Generated frontend files come from this Go backend. Regenerate after any backend contract change.

1. Update Go handlers, response structs, or plugin event definitions.
2. From the **repository root**:
   ```bash
   go generate ./codegen
   ```
3. Commit the regenerated output in `seanime-web/src/api/generated/` and
   `seanime-web/src/app/(main)/_features/plugin/generated/`.

Never `go run ./codegen` — see [`../codegen/README.md`](../codegen/README.md) for why, and for
the full list of what the generator writes.

CI fails when the regenerated output differs from what is committed (`.github/workflows/test.yml`).

### Known codegen constraints

These are load-bearing quirks, not bugs to fix casually — they shape what the generated
types can express.

- **Hand-maintained struct allowlist.** `codegen/internal/generate_types.go` carries
  `additionalStructNames`, ~30 structs that no route handler references directly. A struct
  reachable only through a websocket event or a plugin payload **will be silently missing**
  from `types.ts` until it is added to that list. Nothing warns you; the type simply is not
  emitted.
- **`json.RawMessage` becomes `Record<string, any>`**
  (`codegen/internal/generate_structs.go`). That is one of the few `any` sources in
  otherwise-typed generated output. Go types the mapper does not recognise become `unknown`,
  which is the safe fallback.
- **Emitted optionality is loose.** 917 of 1058 generated fields are optional (`?:`),
  because the generator marks every pointer and `omitempty` field optional. This is why
  frontend consumers reach for `?.` and `!` so often, and why runtime-validating these types
  would buy weaker guarantees than it appears.
- The generated types are a **compile-time contract only**. The frontend does not validate
  API responses at runtime; `seanime-web/src/api/client/requests.ts` casts the response.

## Testing

`go test ./...` does **not** pass on a clean checkout, and that is expected rather than a
regression. CI (`.github/workflows/test.yml`) runs an explicit subset for this reason.

Three separate causes:

- **Missing AniList fixtures.** `test/testdata/**/*.json` is gitignored, so the recorded
  AniList responses are never committed. Affected packages fail with
  `missing AniList fixture for …`. Record them locally with an authenticated client:
  `SEANIME_TEST_RECORD_ANILIST_FIXTURES=true go test ./internal/api/anilist/...`
  Affects: `api/anilist`, `library/anime`, `library/autodownloader`, `library/scanner`, `local`.
- **Missing local config.** `testutil` needs `test/config.toml`, which is gitignored; copy
  `test/config.example.toml` to create it.
- **Failing on main for their own reasons** — genuine pre-existing failures, not
  environmental: `notifier`, `goja/goja_bindings`, `handlers`,
  `platforms/shared_platform` (nil-logger panic in `CacheLayer.checkAndUpdateWorkingState`),
  `platforms/simulated_platform`.

Note that `playlist` passes in isolation but fails in a full `./...` run — the excluded
packages above pollute shared state. Fixing any of these should come with removing it from
the exclusion list in `.github/workflows/test.yml`.

See `../CONTRIBUTING.md` for test-helper conventions (`internal/testmocks/…`).

## Code Quality

- `go vet ./...` and `golangci-lint` (config `../.golangci.yml`); CI scope is described in the
  root `AGENTS.md`.

## Maintaining this file

- Update when backend responsibilities shift or entrypoints change.
- Keep the codegen instructions aligned with `codegen/` and `../seanime-web/AGENTS.md`.
