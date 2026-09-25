# Phase 2b: Relay, web side (seanime-web)

The web app is only a **message bridge**. It never fetches third-party URLs itself.

## Existing pieces to reuse

- **`src/app/websocket-provider.tsx`:** owns the socket. It gets `CLIENT_IDENTITY` (clientId) on
  connect.
- **`src/app/(main)/_hooks/handle-websockets.ts`:**
  - `useWebsocketSender()` (l.13) has a queued `sendMessage`.
  - `useWebsocketMessageListener({ type, onMessage })` (l.285) listens for server events.
- **`src/lib/validation/websocket.ts`:** `parseWebsocketFrame` validates incoming frames. The new
  event must be added there, or the frame is rejected.
- **`src/lib/server/ws-events.ts`:** the `WSEvents` enum. Add `BROWSER_RELAY_REQUEST` and
  `BROWSER_RELAY_STATUS`.
- **Extensions page:** `src/app/(main)/extensions/`, with `_containers/` and `_components/`. The
  failing-extension UI from `891bf5cf` lives there.

## New feature folder `src/app/(main)/_features/browser-relay/`

```
browser-relay/
  bridge-protocol.ts        // page <-> content-script message types, SOURCE constants, version
  use-browser-relay-bridge.ts
  browser-relay.atoms.ts    // extension detected? connected? granted origins? last error
  browser-relay-card.tsx    // settings UI
  __tests__/bridge.test.ts
```

### Page ↔ content-script protocol (`bridge-protocol.ts`)

Messages go over `window.postMessage(msg, window.location.origin)`. Both sides check
`event.source === window`, `event.origin === location.origin`, and the `source` tag.

```ts
// page -> extension
{ source: "seanime-page", v: 1, type: "ping" }
{ source: "seanime-page", v: 1, type: "relay-request", request: RelayRequest }
{ source: "seanime-page", v: 1, type: "request-permissions", origins: string[] }
// extension -> page
{ source: "seanime-relay-ext", v: 1, type: "pong", extVersion, browser, grantedOrigins, modes }
{ source: "seanime-relay-ext", v: 1, type: "relay-response", response: RelayResponse }
{ source: "seanime-relay-ext", v: 1, type: "permissions-result", grantedOrigins }
```

### `useBrowserRelayBridge()`

Mount it once inside the authenticated main layout, next to the other global websocket listeners.

1. **Detect the extension.** On mount and on every websocket reconnect, send `ping` and wait
   500 ms for `pong`. Retry 3 times with backoff; the content script can load after the page.
2. **Announce.** On `pong`, send the client event
   `{ type: "browser-relay", payload: { kind: "hello", …pong } }` through `sendMessage`. On
   `pagehide`, send `bye` (best effort; the server also cleans up on disconnect).
3. **Forward requests.**
   - For each `BROWSER_RELAY_REQUEST` server event, post a `relay-request` to the extension and
     track `id → startedAt`.
   - On `relay-response`, send `{ type: "browser-relay", payload: { kind: "response", … } }`.
   - If there is no extension or no reply within `timeoutMs + 2 s`, reply with
     `error.code = "timeout"` so the server fails fast instead of waiting out its own timer.
4. **Tab election.** Every open Seanime tab says `hello`, and the server picks the most recently
   active one.
   - Send a `hello` refresh (the same payload) on `visibilitychange → visible`, so the focused tab
     wins.
   - Background tabs keep working as fallbacks. Chrome throttles timers in background tabs, but
     message handling and the extension's `fetch` are not throttled.
5. **Scope.** The hook never reads response bodies. It passes them through unchanged, so bodies
   stay opaque and memory stays flat. Don't log bodies.

### Settings card (`browser-relay-card.tsx`)

This goes on the extensions page, above the failing-extension list. It reads
`GET /api/v1/browser-relay/status` and `required-origins`, and listens for `BROWSER_RELAY_STATUS`
for live updates.

| State | Shows | Action |
| --- | --- | --- |
| Extension not detected | "Run extension requests in your browser to avoid bot blocking." | **Install for Chrome** / **Install for Firefox** buttons (store links; picked by UA, with both shown) |
| Detected, tab not connected | Should not happen: detection implies connected. | — |
| Connected, origins missing | "N providers need access to: mangapark.net, …" | **Grant access** posts `request-permissions` with the missing origins. The extension opens the browser prompt. |
| Connected, all granted | Green status, extension version, list of relay-enabled extensions | **Test** calls `POST /browser-relay/test` and shows latency and status |
| Server setting `off` | Greyed out | Link to the setting |

It also shows a 3-way select bound to the global `browserRelay` setting (Off / Auto / Browser
only), saved through the existing extension-settings mutation.

On each extension card, show a small "via browser" badge when the extension has `network.relay`.
The failing-extension panel shows `RelayError` codes (for example `no_permission`) with a
"Grant access" shortcut.

## Tests (Vitest)

- Bridge: ping/pong detection with a mocked `window.postMessage`. Messages from another `source` or
  origin, or from an iframe, are ignored.
- Request forwarding: correlation by ID; the timeout path replies with `timeout`.
- The card's state machine renders the right action for each state.

## Commits

1. `feat(web): browser relay bridge between server events and the companion extension`
2. `feat(web): browser relay settings card and per-extension relay badge`
