# Phase 1: Sync upstream v3.10.3, preferring our implementations

## Upstream delta (`9bdd052a..2da73d9e`, 17 commits, 64 files, +1543/−415)

Headline changes:

- **Date added / last updated sorting:** AniList `createdAt`/`updatedAt` on list entries.
  Touches `internal/api/anilist/queries/*.graphql`, `client_gen.go`, `library/anime/entry*.go`,
  `manga/manga_entry.go`, `collection.go`, and `seanime-web/src/lib/helpers/filtering.ts`.
- **Fixes:**
  - debrid autoplay stale torrent reuse (`6961f38a`)
  - videocore disappearing subs (`954e77ec`)
  - continuity history item deletion (`717bfb3b`)
  - torrentstream `Last-Modified` (`ac6ab21a`)
  - mediacore progress on session replacement (`6c40785e`)
  - mediastream direct play serving the right episode (`b7c8a720`)
- **Plugin API additions:**
  - `internal/plugin/app_settings.go`
  - `internal/plugin/ui/marketplace.go` (+126)
  - `ui/events.go`
  - `plugin.d.ts`/`app.d.ts`
  - generated `plugin-events.ts`
- Go version bump, dependency bumps (`go.mod`/`go.sum`, `seanime-web/package.json`), and `.npmrc`
  files.
- Denshi and `mpv-prism.lock.json` updates. **The fork deleted these files.**

## Predicted conflicts

These come from `git merge-tree --write-tree main upstream/main`. The same set appears against
`harden/server-deployment`, plus `.github/workflows/release-draft-new.yml`, `README.md`, and
`CHANGELOG.md`.

| File | Kind | Resolution rule |
| --- | --- | --- |
| `internal/library/anime/entry.go`, `internal/manga/manga_entry.go` | both added struct fields | **Union.** Keep ours (`Private`, `HiddenFromStatusLists`) and add theirs (`CreatedAt`, `UpdatedAt`). Keep our wider alignment. |
| `internal/library/anime/collection.go`, `internal/manga/collection.go` | both added fields/logic | Union. Keep our hidden-entry filtering as is. |
| `internal/api/anilist/client_gen.go` | generated | Take ours, re-apply the upstream `.graphql` edits, then regenerate (see below). |
| `internal/constants/constants.go` | version | Ours with the version bumped: `Version = "3.10.3-fork.1"`. Keep `BuildTime` and the fork marketplace URL logic. |
| `internal/extension_repo/goja_plugin_types/app.d.ts` | both edited | Union. Keep our extension typings and add the upstream plugin APIs. |
| `mobile/mobile.go` | both edited | Read both. Take the upstream removal only if it doesn't touch fork code. |
| `mpv-prism.lock.json`, `seanime-denshi/package*.json` | modify/delete | **Keep deleted** (`git rm`). The fork dropped Denshi/mpv-prism on purpose. |
| `go.mod`, `go.sum` | deps | Take the higher version of each module, then `go mod tidy`. Keep fork-only deps. |
| `seanime-web/package.json`, `package-lock.json` | deps | Keep our toolchain (Tailwind 4, React Compiler, pins) and take upstream's dependency bumps where they're compatible. Regenerate the lockfile with `npm install`. Never hand-merge a lockfile. |
| `seanime-web/src/api/generated/types.ts`, `codegen/generated/public_structs.json` | generated | Pick either side, then regenerate. |
| `continue-watching.tsx` | both edited | Keep our component and port the upstream sort-option wiring into it. |
| `CHANGELOG.md` | both edited | Keep ours at the top. Insert upstream's `## v3.10.3` section below `## Unreleased`, in version order. |
| `README.md`, `release-draft-new.yml` | fork-owned | Ours. |

Files that auto-merge but deserve a manual look, because fork code lives next to them:
`internal/plugin/ui/marketplace.go` (the fork owns marketplace/extension logic),
`internal/mediacore/mediacore.go`, `internal/directstream/*`, `internal/plugin/videocore.go`, and
`mediastream/page.tsx`.

## Steps

1. **Prep**
   - Start from a clean tree: `git switch main && git pull --ff-only origin main`.
   - `git fetch upstream --tags` has already been done: `upstream/main = 2da73d9e`, tag `v3.10.3`.
2. **Branch:** `git switch -c sync/upstream-v3.10.3`.
3. **Merge without committing:** `git merge --no-ff --no-commit upstream/main`.
   Do **not** use `-X ours` or `-s ours`. A previous "sync ours" merge silently dropped 24 fork
   files; `e0ce5986` had to restore them.
4. **Resolve** each conflict using the table. For every auto-merged file under a fork-owned path,
   review `git diff main -- <file>`.
5. **Deletion guard**
   - Run `git diff --cached --diff-filter=D --name-only main`. The list must be empty, or contain
     only files upstream deliberately removed that the fork doesn't own.
   - Run `git diff --cached --diff-filter=A --name-only main -- seanime-denshi mpv-prism.lock.json`.
     It must be empty.
6. **Regenerate**
   - AniList client: find the generator with `grep -rn "gqlgenc\|genqlient" internal/api/anilist`
     and run it, so `client_gen.go` includes `createdAt`/`updatedAt`.
   - `go generate ./codegen` from the repo root. This regenerates `codegen/generated/`,
     `internal/events/endpoints.go`, `seanime-web/src/api/generated/`, and
     `_features/plugin/generated/`.
   - `go mod tidy`, and `cd seanime-web && npm install`.
   - `routeTree.gen.ts` rebuilds during the `npm run build` check.
7. **Commit**
   - Subject: `chore(merge): integrate upstream 5rahim/seanime v3.10.3`
   - Body: one line per subsystem saying which side won, plus the deleted-file decisions.
8. **Changelog:** add to `## Unreleased`: `🔀 Merged upstream v3.10.3 (date-added/last-updated
   sorting, debrid/videocore/mediastream fixes, plugin marketplace APIs); fork implementations kept
   for …`.
9. **Verify:** see below.
10. **Publish**
    - Push the branch and open a PR to `main`.
    - After it merges, update the working branch: `git switch harden/server-deployment && git merge
      main`. Resolve `CHANGELOG.md`/`README.md` in favor of the branch.

## Verification

- `go build ./... && go vet ./...`
- `golangci-lint run --new-from-merge-base=upstream/main`. This has to be clean for fork-changed
  lines only.
- `go test` with the exclusion list in `internal/AGENTS.md`, "Testing". Also run the new upstream
  tests: `internal/mediacore`, `internal/mediastream`, `internal/handlers` (mediastream),
  `internal/extension_repo` (goja plugin system).
- `go generate ./codegen && git diff --exit-code`, the codegen freshness gate.
- In `seanime-web`:
  - `npx tsgo --noEmit` must report zero errors.
  - `npx vitest run`
  - `npx biome lint --changed`
  - `npm run build`
- Manual smoke test:
  - Start the server and log in.
  - Check the anime and manga library sort menus: the new "Date added" and "Last updated" options
    must work, and hidden/private entries must still be hidden.
  - Check that continue watching works.
  - Play an episode in videocore and confirm subtitles show.
  - Open the extensions page. Check the marketplace and the failing-extension UI (fork features).
