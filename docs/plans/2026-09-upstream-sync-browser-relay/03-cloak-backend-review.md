# Phase 3: cloak-backend review and fixes (seanime-extensions/cloak-backend)

cloak-backend stays in use. It is the headless fallback for background jobs and setups without a
browser, so it must report failures truthfully. Otherwise the fork's `HealthTracker` and the relay
`auto` fallback can't react to them.

## Architecture (as reviewed)

- **Stack:** FastAPI + Uvicorn and a single CloakBrowser (Playwright stealth) instance.
  - Per-origin contexts (`_OriginContext`, `browser_manager.py:~197`) with warm page pools
    (`pages_per_context=3`).
  - Idle contexts are reaped after 900 s.
- **Concurrency:** global semaphore `max_concurrency=2`, plus 150–600 ms jitter.
- **Endpoints** (`app.py`):
  - `GET /health`
  - `POST /request` (fetch or render)
  - `POST /cookies`
  - `POST|GET /image`
  - `POST /reset`
  - Provider RPC: `/providers/{mangapark,mangadex,nhentai,comix}/{search,chapters,pages}`
- **Security:**
  - Binds to `127.0.0.1:47541` by default.
  - Optional `X-Cloak-Token`.
  - Two-layer SSRF guard: a pre-flight check plus a route filter that re-checks redirects and
    subresources (`security.py`).
  - CORS is opt-in.
- **Provider contract:** the request and response shapes in `schemas.py` match what the
  `*-nodriver` JS providers send and parse. No mismatches found.

## Findings and fixes

| # | Sev | Where | Problem | Fix |
| --- | --- | --- | --- | --- |
| 1 | High | `browser_manager.py:648` (`get_cookies`) | `status = resp.status if resp is not None else 200`. A navigation that returns no response is reported as **200 OK**. **Verified.** | Return `status=502, ok=False, error="navigation returned no response"`. The endpoint maps that to HTTP 502. |
| 2 | Med | `_capture_rendered(page, None)`, `~l.854`; called from `submit_form` `~l.730` | Reports `ok=True` even when `interactive_wait_timed_out=True`. Callers have to know to check the flag. | Set `ok = not interactive_wait_timed_out and <nav ok>`, and keep the flag for detail. |
| 3 | Med | `fetch_image` `~l.623–629` vs `app.py:~229` | A missing response gives `status=0` and empty bytes. The endpoint then returns 502 for an empty body but may pass other cases through, so the signal is inconsistent. | `fetch_image` raises a typed `UpstreamError(status, reason)`. The endpoint maps it to one consistent 502 JSON error body. |
| 4 | Low | `fetch_image` `~l.610` | `timeout_ms = self._timeout_ms(None)` ignores the per-context `nav_timeout_ms`. | Decide on it explicitly: use the context timeout, capped at 20 s for images. Add a comment and a test. |
| 5 | ? | Context close | Unverified: a hung `context.close()` in the reaper or at shutdown could block, or leave Chromium children behind. | Wrap it with `asyncio.wait_for(close(), 10)`. On timeout, log it and fall back to `browser.close()` or a process kill at shutdown. Add a test with a fake context that hangs. |
| 6 | ? | Semaphore | Unverified: the semaphore release on client disconnect or cancellation. | Confirm `async with sem:` wraps the whole operation. Add a cancellation test. |
| 7 | ? | SSRF route filter | Unverified: coverage of WebSocket and service-worker requests from a rendered page. | Add `page.route` checks for `websocket` resource types, or block service workers (`service_workers="block"` on the context). |
| 8 | ? | `/cookies`, `/request` exposure | Binding to a non-loopback address without a token exposes a browser-backed SSRF proxy and the clearance cookies. | Refuse to start when `host` is not loopback and no token is set, unless `--insecure-no-token` is passed. |
| 9 | Low | Provider RPC error shape | Check that every `/providers/*` failure returns a non-2xx status, so goja `fetch` → `HealthTracker` sees a failure and not an empty 200 result. | Audit the providers and add tests per endpoint for the upstream-failure path. |

"?" means the item has not been checked against the code yet. Confirm it before changing anything.

## Remote deployment

Remote deployment matters because the relay doesn't cover background jobs.

- Document a compose service next to Seanime in `cloak-backend/docker-compose.yml` and
  `WEB_DEPLOYMENT.md`:
  - The internal network is shared with Seanime.
  - `CLOAK_TOKEN` is set, and Seanime providers pass it through their user config.
  - Don't publish the port.
- Note in the docs that datacenter IPs get challenged more. That's the reason the browser relay is
  the preferred path when a user session exists.

## Steps

1. Write failing tests for #1–#4 in `tests/test_manager.py` and `tests/test_app.py`, with mocked
   Playwright responses returning `None`.
2. Fix them, one commit each: `fix(cloak-backend): …`.
3. Investigate #5–#9. Record each as fixed or not an issue in `cloak-backend/AGENTS.md`, under
   Local Contracts or Known limits.
4. Add the startup refusal (#8) with a CLI flag and a test in `test_config.py`.
5. Update `cloak-backend/AGENTS.md` and `README.md`.

## Verification

- `uv sync && uv run pytest`, or `npm test` from the repo root.
- `npm run dev`, then `uv run python scripts/smoke.py` against the live gateway.
- Manual: point Seanime at the gateway. Use a `*-nodriver` provider with the gateway stopped, then
  force a navigation failure (a bad domain in `/cookies`). The failure must show up in the
  failing-extension UI, not as an empty result.
