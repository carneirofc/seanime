# Phase 2: Browser relay (overview)

## Problem

Extension scraping runs in the Go server's goja runtime:

- **`fetch`:** `internal/goja/goja_bindings/fetch.go` sends requests through
  `clientWithCloudFlareBypass`. That client is `req` with `ImpersonateChrome()`, a fixed Chrome 120
  UA, and a stateless cookie model: cookies are read from each response but never persisted.
- **Headless browser:** `internal/goja/goja_bindings/chromedp.go` exposes up to 5 headless Chrome
  instances. Headless Chrome is also fingerprinted. Only
  `onlinestream-providers/anime-ultra-online-streaming` uses it.
- **The workaround:** the `*-nodriver` manga providers post to cloak-backend
  (`127.0.0.1:47541`), a CloakBrowser/Playwright stealth browser, which then does the scraping.

Both paths scrape from the **server's** IP with a **synthetic** browser identity. Once Seanime is
deployed remotely (datacenter IP), bot detection gets worse.

## Why a plain web page can't do it

The Seanime UI is served from the Seanime origin. A `fetch("https://mangapark.net/...")` from the
page fails in two ways:

- **CORS** blocks reading the response.
- **The target site's cookies** (`cf_clearance`) aren't available to the page.

Neither can be fixed from inside the page. A **browser extension** with host permissions for the
target site isn't subject to CORS, and it sends the user's real cookies, so it can.

## Design in one picture

```
 goja provider                           Go server                            User's browser
 ─────────────                           ─────────                            ──────────────
 fetch(url,{relay:"auto"}) ─► browserrelay.Do ─ SendEventTo(clientId,          Seanime tab (seanime-web)
                                    │          "browser-relay-request") ──►  ws-events listener
                                    │                                            │ window.postMessage
                                    │                                            ▼
                                    │                                        content script (extension)
                                    │                                            │ runtime.sendMessage
                                    │                                            ▼
                                    │                                        service worker: fetch(url)
                                    │                                        (real TLS, cookies, IP)
                                    ◄── client event "browser-relay"  ◄──────────┘ (id-correlated)
                  fallback on no client / timeout / relay error:
                  direct req client (today) → or provider's own cloak-backend path
```

## Why setup stays easy

- **No extra server, port, token, or pairing code.** The relay reuses the Seanime tab's websocket
  (`/events`). That socket is already authenticated by the existing password/OIDC/ticket flow in
  `internal/handlers/websocket.go`, and the server stamps `event.ClientID` itself
  (`websocket.go:~118`), so a response can't be forged by another client.
- **User steps:**
  1. Install the extension from the Chrome Web Store / AMO. The link is on the Seanime extensions
     page.
  2. On the Seanime tab, click the extension icon → **Connect this Seanime tab**. This grants that
     one origin and works the same for `http://127.0.0.1:43211` and `https://seanime.example.com`.
  3. On the Seanime extensions page, click **Grant access for enabled providers**. One browser
     prompt lists the domains.

  After that the relay turns on automatically whenever a Seanime tab is open.

## Sub-phases

- [02a-relay-server.md](02a-relay-server.md): Go `internal/browserrelay`, goja `fetch` option,
  events, settings.
- [02b-relay-web.md](02b-relay-web.md): the seanime-web bridge and settings card.
- [02c-relay-extension.md](02c-relay-extension.md): the MV3 WebExtension: permissions, fetch,
  challenge solving, distribution.
- [02d-relay-providers.md](02d-relay-providers.md): pilot provider, images, benchmark, migration.

## Wire protocol (v1)

Every message carries `v: 1`. JSON over the existing websocket, as `WSEvent` / `WebsocketClientEvent`.

**Server → client**, event type `browser-relay-request`:

```jsonc
{ "v":1, "id":"<128-bit hex>", "extensionId":"mangapark-relay",
  "url":"https://…", "method":"GET", "headers":{"Referer":"…"},
  "body":null | "<base64>", "mode":"fetch" | "render",
  "render": { "waitFor":"selector" | null, "timeoutMs":20000 } | null,
  "timeoutMs":30000, "responseType":"text" | "binary" }
```

**Client → server**, client event `type: "browser-relay"`:

```jsonc
// payload.kind ∈ hello | response | bye
{ "kind":"hello", "v":1, "extVersion":"1.0.0", "browser":"chrome",
  "grantedOrigins":["https://mangapark.net/*"], "modes":["fetch","render"] }
{ "kind":"response", "id":"…", "ok":true, "status":200, "statusText":"OK",
  "url":"<final url after redirects>", "headers":{…},
  "body":"<utf8 or base64>", "bodyEncoding":"utf8"|"base64",
  "challenge": null | "solved" | "unsolved",
  "error": null | { "code":"no_permission"|"timeout"|"network"|"too_large"|"blocked", "message":"…" } }
{ "kind":"bye" }
```

## Non-goals and limits

- **No background jobs without a browser.** Scheduled and headless work (auto-downloader, library
  refresh) can't use the relay when no tab is open. It falls back to direct fetch or cloak-backend.
- **Mobile:** only Firefox for Android supports extensions. Other mobile browsers fall back.
- **Latency:** each relayed request adds one websocket round trip (a few ms locally, tens of ms
  remote). That is offset by avoiding challenge failures and retries.
- **Not a general proxy:** only extensions that opt in, and only domains they already declare in
  `allowedDomains`, can use the relay.
