# Seanime Agents

Root file = coordination index for the repo. Subproject ownership + workflows live in the
nearest `AGENTS.md`.

## Subprojects

- Go backend: `internal/AGENTS.md`.
- Frontend: `seanime-web/AGENTS.md`.

## Scope

- Repo-level coordination only.
- Update the subproject `AGENTS.md` when responsibilities, entrypoints, or local workflows change.

## Fork context

This repository is a fork that tracks upstream `5rahim/seanime`. That constraint shapes the
quality gates below: **this fork's own changes are held to the full standard, while the
inherited upstream backlog is not a merge blocker.** Changes that rewrite files wholesale
(a formatter, a broad regeneration) conflict with every upstream pull and are avoided.

## Code quality

Conventions for contributors — coding style, testing helpers, PR expectations, AI
disclosure — live in `CONTRIBUTING.md`. This section covers only the automated gates.

| Gate | Where | Scope |
| --- | --- | --- |
| `go vet`, `golangci-lint` | `.github/workflows/lint.yml` | Lines this fork changed (`--new-from-merge-base=upstream/main`) |
| TypeScript typecheck (`tsgo`) | `.github/workflows/lint.yml` | Whole frontend; must stay at zero |
| Biome lint | `.github/workflows/lint.yml` | Files changed in the PR only |
| `go test` | `.github/workflows/test.yml` | All packages except a documented exclusion list |
| Vitest | `.github/workflows/test.yml` | Whole frontend |
| Codegen freshness | `.github/workflows/test.yml` | Fails if `go generate ./codegen` changes the tree |
| Vulnerability + secret scanning | `.github/workflows/security.yml` | Whole repo, plus a weekly schedule |

Two gates are deliberately narrower than they look, and both are documented where they live:

- **`go test` excludes 11 packages.** Some need AniList fixtures or a local config that are
  gitignored by design; others fail on `main` for their own reasons. See "Testing" in
  `internal/AGENTS.md` for the breakdown and how to run them locally.
- **Biome lints only files changed in a PR.** Its `--changed` works at file granularity, not
  line granularity like golangci-lint, so a wider base would re-flag the whole inherited
  tree. Noisy rules with large backlogs are switched off with counts recorded in
  `seanime-web/biome.jsonc`. The formatter is configured but intentionally not enforced.

Not enforced yet, with the cost of each measured in `seanime-web/tsconfig.json`:
`noUncheckedIndexedAccess`, `noUnusedLocals`, `noUnusedParameters`.

## Generated files

Never hand-edit. Regenerate and commit instead.

- `codegen/generated/`, `seanime-web/src/api/generated/`,
  `seanime-web/src/app/(main)/_features/plugin/generated/` — from `go generate ./codegen`
  (run from the repo root; see `internal/AGENTS.md` for why `go run ./codegen` is wrong).
- `seanime-web/src/routeTree.gen.ts` — from the TanStack Router plugin during dev/build.
