# Seanime Backend Agents

Go backend powering Seanime API and serving embedded web UI. Covers runtime slices, entrypoints, and how backend changes flow to frontend.

## At-a-glance

| Agent | Scope | Key Paths | Runtime | Typical Commands |
| --- | --- | --- | --- | --- |
| Core Server | Boot, flags, config, logging, updater | `main.go`, `internal/server/`, `internal/core/` | Go 1.24+ | `go run .`, `go test ./...`, `go build -o seanime` |
| HTTP API + Events | REST endpoints and websocket events | `internal/handlers/`, `internal/core/echo.go` | Echo v4 | `go test ./...` |
| Embedded Web UI | Serve built React SPA | `web/`, `internal/core/echo.go` | Go embed FS | Build web, then copy into `web/` |
| Background Jobs | Recurring sync/update loops | `internal/cron/` | Go | Runs with server startup |

## Core Server

- `main.go` embeds `web/` and `internal/icon/logo.png`, then calls `internal/server.StartServer`.
- OS-specific server entrypoints in `internal/server/server_{unix,windows}.go`.
- Flags parsed in `internal/core/app.go`; config and logging in `internal/core/config.go` and `internal/util/`.

## HTTP API + Events

- Echo instantiated in `internal/core/echo.go` with JSON serialization overrides.
- Routes registered in `internal/handlers`, mounted under `/api`.
- Event streams served under `/events` (websocket/SSE handlers in `internal/handlers`).

## Embedded Web UI

- Static web UI embedded via `//go:embed all:web` in `main.go`.
- `internal/core/echo.go` mounts embedded `web/` filesystem, uses HTML5 fallback routing.
- Static mounts from config: `/assets` → `app.Config.Web.AssetDir`, `/manga-downloads` → `app.Config.Manga.DownloadDir`, `/offline-assets` → `app.Config.Offline.AssetDir`.
- Don't edit `web/` by hand; build artifact from `seanime-web/`.

## Backend → Frontend Codegen

Generated frontend files come from Go backend; regenerate after backend contract changes.

Steps:
1. Update Go handlers, response structs, or plugin event definitions.
2. From repo root, run:
   ```
   go run ./codegen
   ```
   (or `go generate ./codegen`).
3. Commit regenerated outputs in `seanime-web/src/api/generated/` and `seanime-web/src/app/(main)/_features/plugin/generated/`.

## Maintaining this file

- Update `internal/AGENTS.md` when backend responsibilities shift or entrypoints change.
- Keep codegen instructions aligned with `codegen/` outputs and frontend expectations.
