# Phase 2a: Relay, server side (Go)

## Existing pieces to reuse

- **`internal/events/websocket.go`**
  - `SendEventTo(clientId, type, payload)` (l.164) targets one client.
  - `OnClientEvent` (l.227) fans client events out to subscriber maps chosen by
    `WebsocketClientEventType`.
  - Subscribers are buffered channels that **drop events when full** (l.241), which is not
    acceptable for request/response.
- **`internal/handlers/websocket.go`**
  - The read loop (l.104) stamps `event.ClientID = id` on every client event, so the server
    authenticates who replied.
  - It has no `SetReadLimit`, so message size is unbounded today.
- **`internal/goja/goja_bindings/fetch.go`**
  - `Fetch.Fetch` (l.280) is the JS `fetch` binding, and `isURLAllowed` (l.207) holds the domain
    rules.
  - Provider extensions are bound with **no** allowlist (`internal/extension_repo/goja_base.go:79`,
    `BindFetch(ext.ID, vm)`), which means they can reach any domain. Plugins pass
    `GetNetworkAccessAllowedDomains()`.
- **`internal/extension_repo/health.go`:** `HealthTracker.Record(id, err)` (l.57), the
  runtime-health tracking added in `c37af230`.
- **`internal/extension_repo/external_settings.go`:** `StoredExtensionSettingsData`, which holds
  `AutoDisableFailing`.

## New package `internal/browserrelay`

```
internal/browserrelay/
  relay.go        // Relay type, Do(), registry, lifecycle
  protocol.go     // Request/Response/Hello structs, codes, v=1
  challenge.go    // challenge heuristics shared with tests
  relay_test.go
```

### Types

```go
type Request struct {
    ExtensionID string            `json:"extensionId"`
    URL         string            `json:"url"`
    Method      string            `json:"method"`
    Headers     map[string]string `json:"headers,omitempty"`
    Body        []byte            `json:"body,omitempty"`    // base64 on the wire
    Mode        string            `json:"mode"`              // "fetch" | "render"
    Render      *RenderOptions    `json:"render,omitempty"`
    Timeout     time.Duration     `json:"-"`
}
type Response struct { Status int; StatusText, URL string; Headers map[string]string; Body []byte; Challenge string }
type ClientInfo struct { ClientID, ExtVersion, Browser string; GrantedOrigins []string; Modes []string; LastSeen time.Time }
```

### Relay

- **`New(ws events.WSEventManagerInterface, logger)`**
  - Subscribes once with a **dedicated** subscriber map: add `BrowserRelayEventType
    WebsocketClientEventType = "browser-relay"` in `internal/events/events.go` and route it in
    `OnClientEvent`, like `PlaylistEvent`.
  - The channel is buffered at 256. A single goroutine drains it and dispatches `hello`, `response`,
    and `bye`.
- **Registry:** `map[clientID]*ClientInfo` under a mutex.
  - `hello` inserts or updates an entry.
  - `bye` and socket disconnects remove it. For disconnects, add a hook in
    `WSEventManager.RemoveConn`: an `OnDisconnect(func(id string))` list. That is a small, additive
    change.
- **`Available(url string) bool`** reports whether some relay client has a granted origin that
  matches the host.
- **`Do(ctx, req) (*Response, error)`**
  1. Validate first:
     - The scheme is http or https.
     - The host is not a private or loopback address. Reuse `security.ValidateOutboundUrl`, the same
       as the direct path does.
     - The host is in the extension's relay domains (see the manifest section).
  2. Pick a client: the most recent `LastSeen` among clients whose `GrantedOrigins` match the host
     and whose `Modes` include `req.Mode`. If there is none, return `ErrNoClient`.
  3. `id := hex(crypto/rand 16 bytes)`. Store `pending[id] = {clientID, ch chan *Response}`.
  4. `ws.SendEventTo(clientID, events.BrowserRelayRequest, wirePayload, true)`, with `noLog` so URLs
     don't leak into the logs.
  5. `select` on the response, `ctx.Done()`, and the timeout. The default timeout is 30 s, 90 s when
     a challenge is involved, and at most 120 s.
  6. On a response, check `resp.clientID == pending.clientID`; otherwise drop it and log a warning.
     Then delete `pending[id]`.
  7. Enforce the body limit after decoding: `MaxBody = 16 << 20`. Return `ErrTooLarge` beyond it.
- **Concurrency:** a semaphore of 8 per client, so one tab isn't flooded. Requests beyond that queue
  until ctx or timeout.
- **Size safety:** add `ws.SetReadLimit(24 << 20)` in `internal/handlers/websocket.go`. That caps
  base64 bodies (16 MB × 4/3) plus framing. Check existing large client events (playlist, nakama)
  against this cap; they are far below it.

### Events

These go in `internal/events/events.go`. Mirror the constants in
`seanime-web/src/lib/server/ws-events.ts`, then run `go generate ./codegen`.

- `BrowserRelayRequest = "browser-relay-request"` (server → client)
- `BrowserRelayStatus = "browser-relay-status"` (server → all clients). It is sent when the registry
  changes so the settings card updates live.
- `BrowserRelayEventType = "browser-relay"`, the client-event type.

### App wiring

- Construct the relay in `internal/core` next to `WSEventManager`, and expose it as
  `App.BrowserRelay`.
- Pass it into the extension repository (`internal/extension_repo`) so `BindFetch` can take it.

### HTTP endpoints (`internal/handlers/browser_relay.go`)

These need codegen annotations like the other handlers.

- `GET /api/v1/browser-relay/status` → `{ clients: ClientInfo[], enabledMode }`
- `GET /api/v1/browser-relay/required-origins` → the union of relay domains of enabled extensions
  that opted in. The settings card uses it to build the permission request.
- `POST /api/v1/browser-relay/test` → `{ url }`. Relays a GET to a harmless URL, for example the
  first origin of an enabled relay provider, and returns status and latency.

## Extension manifest opt-in

Add to `extension.Extension` in `internal/extension/extension.go`:

```go
// Network configures how the extension's fetch reaches the internet.
Network *NetworkConfig `json:"network,omitempty"`

type NetworkConfig struct {
    // Relay: "off" (default) | "prefer-browser" | "require-browser"
    Relay string `json:"relay,omitempty"`
    // RelayDomains: hosts the relay may fetch for this extension (same pattern syntax as allowedDomains).
    RelayDomains []string `json:"relayDomains,omitempty"`
}
```

- `RelayDomains` is **required** before any relaying happens. It scopes relay traffic even though
  provider extensions otherwise have unrestricted `fetch`. It is also the list the browser
  extension asks the user to grant.
- Validate it on install in the manifest checks under `internal/extension_repo`:
  - no `*` wildcard
  - no private hosts
  - at most 20 entries
- Update the typings everywhere they are vendored: `internal/extension_repo/goja_plugin_types/*.d.ts`,
  and the shared `*.d.ts` in the seanime-extensions repo.

## goja `fetch` integration (`fetch.go`)

- **New option**, per request:
  `fetch(url, { relay?: "auto" | "browser" | "off", render?: { waitFor?: string, timeoutMs?: number } })`.
- **Resolution order:**
  1. The per-request option.
  2. The manifest `network.relay`: `prefer-browser` → `auto`, `require-browser` → `browser`.
  3. The global setting (below).
  4. `off`.
- **Behavior:**
  - **`off`:** today's path, unchanged.
  - **`browser`:** `Relay.Do`. Errors are thrown to JS as `RelayError` with a `code`.
  - **`auto`:** relay if `Relay.Available(url)`. On `ErrNoClient`, timeout, or an `unsolved`
    challenge, fall back to `clientWithCloudFlareBypass`. Log the fallback at debug level with the
    reason.
  - **`render`:** allowed only through the relay. Without a client, `auto` falls back to a plain
    fetch and sets `response.rendered = false` so the provider can tell.
- **Response mapping:** build the same JS `Response`-like object the direct path returns: `status`,
  `ok`, `headers`, `text()`, `json()`, `arrayBuffer`. Add `response.via = "browser" | "direct"` for
  diagnostics.
- **VM thread safety:** keep to the existing pattern. The work runs in a goroutine and the result is
  delivered through `vmResponseCh`, the same as the direct path, so the goja VM is never touched off
  its loop.
- **Health:** relay errors go through the same error path, so `HealthTracker.Record` counts them.
  `ErrNoClient` under `browser` mode counts as a failure. A fallback that then succeeds counts as a
  success.

## Global setting

Add to `StoredExtensionSettingsData` (`external_settings.go`):

```go
BrowserRelay string `json:"browserRelay,omitempty"` // "off" | "auto" (default) | "browser"
```

- `off` turns the relay off everywhere, overriding manifests.
- `auto` honors manifests.
- `browser` treats `prefer-browser` like `require-browser`, for users who never want server-IP
  scraping.

## Tests (`relay_test.go`, with `events.MockWSEventManager`)

- Happy path: the request goes out and the response is correlated by ID.
- A response from the wrong client is ignored, and the request times out.
- Timeout returns an error and cleans up `pending`.
- An oversize body returns `ErrTooLarge`.
- No client returns `ErrNoClient`, and `auto` falls back.
- A disconnect removes the client, and in-flight requests fail fast.
- Domain checks: a private host and a host outside `RelayDomains` are both refused.
- fetch option resolution matrix, table-driven, for setting × manifest × per-request.

## Commits

1. `feat(events): add browser-relay client event channel and disconnect hook`
2. `feat(browserrelay): relay extension requests through a connected browser`
3. `feat(extensions): network.relay manifest field and relayDomains validation`
4. `feat(goja): route fetch through the browser relay with direct fallback`
5. `feat(handlers): browser relay status, required-origins and test endpoints`
6. `chore(codegen): regenerate`
