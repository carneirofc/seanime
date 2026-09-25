# Phase 2d: Providers, images, benchmark, migration (seanime-extensions)

## Current state

- **Gateway providers:** `manga-providers/*-nodriver` (comix, mangapark, mangadex, nhentai) post to
  `{{nodriverApiBaseUrl}}/providers/<name>/{search,chapters,pages}` on cloak-backend. The scraping
  logic lives in **Python**: `cloak-backend/cloak_backend/providers/<name>.py` and `schemas.py`.
- **Direct providers:** for example `manga-providers/mangabats/provider.js` calls `fetch(url)`
  directly from goja with a `Referer` header.
- **Images:** chapter page images from nodriver providers carry `X-Seanime-Nodriver-*` headers and
  are fetched through cloak-backend `/image`, or through Seanime's `/api/v1/image-proxy`.

## Pilot: `mangapark-relay`

1. **Port the scraping to JS.** Port `cloak_backend/providers/mangapark.py` into
   `manga-providers/mangapark-relay/provider.ts`, following the `manga-provider.d.ts` contract.
   - Use `fetch(url, { relay: "auto" })`.
   - Parse with `LoadDoc` (goja cheerio-like binding) the same way direct providers do.
   - If mangapark needs JS-rendered content for chapter pages, use `render: { waitFor: "…" }` for
     that call only.
2. **Write the manifest** `mangapark-relay.json`:
   ```jsonc
   { "id": "mangapark-relay", "type": "manga-provider", "language": "typescript",
     "network": { "relay": "prefer-browser", "relayDomains": ["mangapark.net", "*.mpcdn.org"] },
     "payloadURI": "provider.ts", … }
   ```
   Confirm the real image CDN hosts from the Python provider before listing them.
3. Register the provider in `marketplace.json`. Don't add it to `default.json` until the benchmark
   passes.
4. Keep `mangapark-nodriver` as it is. The two can be installed side by side for comparison.

## Images: don't push megabytes through the websocket

Page images are the bulk of the bytes. Options, in order of preference:

1. **Load images directly in the browser (preferred).**
   - The provider returns page URLs with `headers: { Referer }`, the same as today's
     `ChapterPage.headers`.
   - The extension installs a **dynamic** `declarativeNetRequest` rule: for each granted relay
     domain, requests initiated from connected Seanime origins get the `Referer` the provider
     declared.
     - To pass the per-provider referer, the web reader posts `{type:"image-referer", host, referer}`
       to the extension when a chapter opens. The extension then adds a session rule scoped by
       `initiatorDomains: [seanime host]` and `requestDomains: [cdn host]`.
   - The reader then uses plain `<img src="https://cdn…">`: zero server bandwidth, and the images
     benefit from the user's CDN cache and cookies.
   - **Web change:** in the manga reader's image URL builder, when the relay is connected and the
     provider has `network.relay`, skip `/api/v1/image-proxy` and use the direct URL.
2. **Server image proxy fallback:** when no relay is connected, the existing `/api/v1/image-proxy`
   path is used unchanged. It works for same-machine deployments and for CDNs that don't
   fingerprint.
3. **Relay-fetched images (last resort):** only for CDNs that challenge image requests. Fetch them
   through the relay with `responseType: "binary"`, capped at 16 MB each.

## Benchmark (gate for migrating the rest)

Script `scripts/bench-relay.ts` in seanime-extensions. It calls Seanime's manga provider endpoints,
so it measures the real path.

- **Matrix:** 3 paths × 3 operations × 20 queries.
  - Paths: `mangapark-relay` (browser connected), `mangapark-nodriver` (cloak-backend),
    `mangapark-relay` with `browserRelay=off` (direct fetch).
  - Operations: search, chapters, pages.
- **Record:** success rate, p50/p95 latency, challenge count, and for images the bytes through the
  server.
- **Run in two setups:** same machine, and remote (docker-compose behind Caddy, `example.Caddyfile`,
  on a VPS or a second host).
- **Pass criteria:**
  - relay success ≥ cloak-backend success
  - relay p95 ≤ 1.5 × cloak-backend p95
  - zero bytes of images through the server when connected
- Save the results in `docs/plans/2026-09-upstream-sync-browser-relay/bench-results.md` in the
  seanime repo.

## Migration (only after the benchmark passes)

1. Port `comix`, `mangadex`, and `nhentai` the same way.
   - `mangadex` is a public JSON API that is rarely challenged. It may only need `relay: "auto"` to
     use the user's IP, or a plain direct fetch.
2. Direct providers that are often blocked (check `HealthTracker` failure data from the fork's
   failing-extension UI) get `network.relay: "prefer-browser"` plus `relayDomains`. That is a
   manifest-only change, because the default `auto` fetch handles the rest.
3. Update `default.json` to prefer the `-relay` variants. Keep the `-nodriver` variants listed for
   headless setups.
4. **Deduplicate:** once the relay variants are stable, the Python `providers/*.py` RPC endpoints
   can be deprecated. cloak-backend keeps its generic `/request`, `/cookies`, and `/image` for
   headless setups. Don't delete anything before at least one release cycle.

## Commits (seanime-extensions)

1. `feat(manga-providers): mangapark-relay provider using the browser relay`
2. `chore(marketplace): register mangapark-relay`
3. `test(bench): relay vs nodriver vs direct benchmark script`
4. (after the gate) `feat(manga-providers): relay variants for comix, mangadex, nhentai`
