# Phase 4: Docs, AGENTS.md pass, closeout

## AGENTS.md updates

| File | Change |
| --- | --- |
| `AGENTS.md` (root) | Add `browser-relay/` to Subprojects. Add a "Browser relay" row to the Code quality table (Vitest + build in the new CI workflow). |
| `internal/AGENTS.md` | Add the `internal/browserrelay` package: purpose, the `Do` contract, limits, and the tests. Document the goja `fetch` `relay`/`render` options and the `network` manifest field. |
| `seanime-web/AGENTS.md` | Add the `_features/browser-relay/` folder. `bridge-protocol.ts` is the source of truth for the extension protocol. |
| `browser-relay/AGENTS.md` (new) | Purpose, build and test commands, store links, the security checklist, and the protocol-sync rule. |
| seanime-extensions `AGENTS.md` | Add the `network.relay` / `relayDomains` manifest contract and the `-relay` provider convention. The layout table gains no new directories. |
| `cloak-backend/AGENTS.md` | The new error contract (non-2xx on failure), the startup refusal without a token, and the remote compose recipe. |

## User docs

- **`WEB_DEPLOYMENT.md`:** a new "Browser relay" section.
  - What the relay does, and why it helps remote deployments (the user's IP, not the datacenter's).
  - Setup in 3 steps (install → Connect → Grant access).
  - What still uses the server's IP (background jobs), and how to add cloak-backend for those.
  - Security note: only grant scraping domains.
- **`browser-relay/README.md`:** the store listing text, the permission rationale, and developer
  load-unpacked instructions.

## CHANGELOG.md (`## Unreleased`)

Use one entry per phase, in the existing emoji style:

- `🔀 Merged upstream v3.10.3 …`
- `✨ Browser relay: extensions can fetch through your browser session (companion extension) to avoid bot detection; falls back to direct fetch/cloak-backend`
- `✨ network.relay / relayDomains extension manifest fields; fetch relay/render options`
- `🦺 Websocket read limit; relay requests scoped to declared domains`

seanime-extensions has no `CHANGELOG.md`. Suggest creating one, per the global rule, and don't
create it silently.

## Final verification

This is the full gate set from the root `AGENTS.md`:

- `go vet`, `golangci-lint --new-from-merge-base=upstream/main`, `go test` (with exclusions)
- `go generate ./codegen && git diff --exit-code`
- seanime-web: `tsgo`, Vitest, `biome lint --changed`, `npm run build`
- `browser-relay`: `npm test && npm run build`
- seanime-extensions: `npm test` (pytest)
- The end-to-end relay scenario from `02c` and the benchmark from `02d`.

## Report back

- For each phase: what landed (commits and PRs), gates run and results, and anything skipped with
  the reason.
- Docs deliberately left unchanged, with the reason.
