# Plan: upstream sync, browser relay, cloak-backend review

Written 2026-09-25 on branch `harden/server-deployment`.

## Why

1. `upstream/main` (5rahim/seanime) has moved on to **v3.10.3** (`2da73d9e`), 17 commits past the
   last merged upstream commit (`9bdd052a`). We want those changes, but our implementations win
   wherever the two sides overlap.
2. Extensions scrape from the Go server through goja `fetch`
   (`internal/goja/goja_bindings/fetch.go:36`). That client uses `req`, impersonates Chrome's TLS,
   hardcodes a Chrome 120 UA, and has no cookie jar. Cloudflare and DDoS-Guard catch it. Today's
   workaround is the Python `cloak-backend` gateway (`seanime-extensions/cloak-backend`). It needs
   a stealth browser running next to the server, and it scrapes from the server's IP.
3. The Python gateway API needs a review, and it has at least one confirmed error-handling bug.

## Outcome

- The fork is on upstream v3.10.3 and keeps all fork behavior.
- A **browser relay** lets extension requests run in the user's real browser session, with its
  real TLS fingerprint, real cookies (including `cf_clearance`), and the user's IP. Setup is
  install → one click → grant. It works whether Seanime runs on localhost or on a remote server.
  When no browser is connected, requests fall back to direct fetch or cloak-backend.
- cloak-backend reports failures truthfully and is documented for remote deployments.

## Phases

| # | Doc | Repo | Depends on |
| --- | --- | --- | --- |
| 1 | [01-upstream-sync.md](01-upstream-sync.md) | seanime | — |
| 2 | [02-browser-relay-overview.md](02-browser-relay-overview.md) | both | 1 |
| 2a | [02a-relay-server.md](02a-relay-server.md) | seanime (Go) | 1 |
| 2b | [02b-relay-web.md](02b-relay-web.md) | seanime (web) | 2a |
| 2c | [02c-relay-extension.md](02c-relay-extension.md) | seanime (`browser-relay/`) | 2b protocol |
| 2d | [02d-relay-providers.md](02d-relay-providers.md) | seanime-extensions | 2a–2c |
| 3 | [03-cloak-backend-review.md](03-cloak-backend-review.md) | seanime-extensions | — (parallel) |
| 4 | [04-docs-closeout.md](04-docs-closeout.md) | both | all |

Phase 3 is independent of the others and can start right away.

## Ground rules (from AGENTS.md / CLAUDE.md)

- Use Conventional Commits, one logical step per commit, and never add a co-author trailer.
- Every change gets a `CHANGELOG.md` entry under `## Unreleased`, using the file's emoji style.
- Never hand-edit generated files. Run `go generate ./codegen` from the repo root.
- Do not use wholesale rewrites or formatters: they conflict with every future upstream pull.
- Do an AGENTS.md pass at the end of every phase.
