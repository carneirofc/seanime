# Phase 2c: Companion browser extension (`browser-relay/`)

## Placement

- **Location:** new top-level folder `browser-relay/` in this repo. The protocol is versioned with
  the server and web code, so they change together.
- **One codebase for Chrome (MV3) and Firefox (MV3, 128+).** Build-time manifest tweaks cover the
  differences: Firefox uses `background.scripts` plus `browser_specific_settings.gecko.id`.
- **Tooling:** TypeScript, a small `esbuild` build script, Vitest, and `webextension-polyfill` for
  the `browser.*` promise API. No framework: the popup is plain HTML/TS.

```
browser-relay/
  AGENTS.md
  package.json            // build, test, zip:chrome, zip:firefox
  build.mjs               // esbuild bundles + per-browser manifest emit
  manifest.base.json
  src/
    background.ts         // service worker: permissions, fetch, render, challenge, DNR rules
    content.ts            // bridge between the Seanime page and background
    popup.html / popup.ts // "Connect this Seanime tab", status, disconnect
    lib/protocol.ts       // shared with seanime-web bridge-protocol.ts (copied at build; see below)
    lib/guard.ts          // host allowlist + private-IP checks
    lib/challenge.ts      // Cloudflare / DDoS-Guard detection
    lib/headers.ts        // forbidden-header handling via declarativeNetRequest
  test/*.test.ts
```

**Protocol sharing:** `seanime-web/src/app/(main)/_features/browser-relay/bridge-protocol.ts` is
the source of truth. `build.mjs` copies it into `src/lib/protocol.ts`, and a Vitest check fails if
the two diverge. That avoids adding a workspace package to the repo.

## Manifest (base)

```jsonc
{
  "manifest_version": 3,
  "name": "Seanime Browser Relay",
  "version": "1.0.0",
  "description": "Lets your Seanime server fetch extension sources through this browser.",
  "permissions": ["storage", "scripting", "declarativeNetRequestWithHostAccess", "notifications", "tabs"],
  "optional_host_permissions": ["<all_urls>"],
  "host_permissions": [],
  "background": { "service_worker": "background.js", "type": "module" },
  "action": { "default_popup": "popup.html" },
  "icons": { … }
}
```

- The extension is granted nothing at install. The store listing shows no host access, which makes
  review easier and builds trust.
- `tabs` is needed only to open and close challenge tabs. If store review objects, `activeTab` plus
  `windows.create` can replace it.

## Setup flow

1. **Install.** The Seanime settings card links to the Chrome Web Store (unlisted listing) and AMO
   (unlisted, signed). Release zips are also attached to fork GitHub releases for manual install.
2. **Connect.** The user opens Seanime and clicks the toolbar icon. The popup detects the active
   tab's origin and shows **Connect this Seanime tab (http://127.0.0.1:43211)**. Clicking it:
   1. Calls `permissions.request({ origins: [origin + "/*"] })`. This runs from a user gesture in
      the popup, as the API requires.
   2. Calls `scripting.registerContentScripts([{ id: "seanime-" + hash(origin), matches: [origin + "/*"], js: ["content.js"], runAt: "document_start", persistAcrossSessions: true }])`.
   3. Injects immediately into the current tab with `scripting.executeScript`, so no reload is
      needed.
   4. Stores `{ origin, connectedAt }` in `storage.local`. Several Seanime origins are allowed, for
      example localhost and a remote host.
3. **Grant.** When the page posts `request-permissions`, the content script forwards it. The
   background opens a small extension page (`grant.html?origins=…`) in a popup window. It lists
   the domains and has one **Allow** button that calls `permissions.request`. A popup window is
   needed because `permissions.request` requires a user gesture inside an extension page, and the
   gesture on the Seanime page doesn't carry over. The result goes back as `permissions-result`.
4. **Done.** From then on the content script loads on every visit to that Seanime origin and the
   relay connects on its own.

**Pre-offer:** if the active tab is `localhost` or `127.0.0.1` on port 43211, the popup
highlights Connect. There is no auto-injection without a click.

**Disconnect:** the popup lists connected origins with a remove (✕) button, which unregisters the
script and removes the permission.

## Content script (`content.ts`)

- It only relays messages. It doesn't touch the DOM or read any page data.
- It handles `window.message` events that come from the same window and origin with
  `source: "seanime-page"`, and forwards them to the background with `runtime.sendMessage`.
- It posts background replies back to the page with `postMessage(msg, location.origin)`.
- **`pong` on `ping`:** the reply includes the extension version, the browser, the granted origins
  (from `permissions.getAll()` through the background), and the supported modes.

## Background (`background.ts`)

### `fetch` mode

1. **Guard (`guard.ts`):**
   - The URL is http or https.
   - The host is not an IP literal in a private, loopback, or link-local range, and not `localhost`.
     A DNS-level check isn't possible from an extension; the server also validates before
     relaying.
   - `permissions.contains({ origins: [originOf(url) + "/*"] })`, otherwise return the error
     `no_permission`.
   - The request came from a registered Seanime origin: check `sender.origin` against the stored
     connected origins.
2. **Headers:**
   - Normal headers go straight into `fetch`.
   - Forbidden or overwritten headers (`Referer`, `Origin`, `User-Agent`, `Cookie`) are set with a
     temporary **session** `declarativeNetRequest` rule:
     `{ condition: { urlFilter: "|" + url + "|", tabIds: [-1] (extension-initiated) }, action: modifyHeaders }`.
     The rule is added before the fetch and removed after it.
   - `User-Agent` is **never** overridden. The browser's real UA is the whole point.
   - `Cookie` from the provider is ignored. The browser's cookie jar is used.
3. `fetch(url, { method, headers, body, credentials: "include", redirect: "follow", signal: AbortSignal.timeout(timeoutMs) })`.
4. **Body:**
   - Read as `arrayBuffer`. If it's larger than 16 MB, return `too_large`.
   - For `responseType: "text"`, or a text-like content type, send UTF-8. Otherwise send base64.
5. **Challenge detection (`challenge.ts`):** status 403, 429, or 503, plus any of:
   - a `cf-mitigated: challenge` header
   - `<title>Just a moment...`
   - `challenge-platform` in the body
   - DDoS-Guard markers

   If found, go to challenge solving.

### Challenge solving

1. **One solver at a time per origin.** Other requests for that origin wait on the same promise.
2. **Open the tab.** `tabs.create({ url, active: false })`, or
   `windows.create({ url, type: "popup", focused: false, width: 500, height: 700 })` on Firefox.
3. **Poll.** Every 1 s, for up to 25 s, check `cookies.get({ url, name: "cf_clearance" })` and the
   tab title. Solved means the clearance cookie is present, or the title no longer matches the
   challenge.
4. **If not solved (interactive captcha):**
   - Focus the window.
   - Show a notification: "Seanime needs you to complete a check on mangapark.net".
   - Wait up to 90 s in total.
5. **Close** the tab or window and **retry the fetch once**. Report `challenge: "solved"`, or
   `"unsolved"` with the last response. The server's `auto` mode then falls back.

The clearance lives in the user's normal cookie jar. Later relayed fetches, and the user's own
browsing, reuse it.

### `render` mode (phase 2c-ii, after the pilot proves `fetch`)

1. Reuse one hidden **offscreen-style tab** per origin: `tabs.create({ active: false })`, kept
   warm for 5 min.
2. Navigate with `tabs.update`. Wait for `waitFor` (a selector, checked by `scripting.executeScript`
   polling) or for `complete` plus 500 ms.
3. Return `document.documentElement.outerHTML` through `executeScript`. This covers what
   cloak-backend's render mode (`/request` with render) does today.

Chrome's `offscreen` documents can't navigate to third-party pages with cookies, so a real tab is
required.

### Limits

- At most 6 concurrent fetches.
- At most 1 concurrent render per origin.
- In-memory LRU of recent `no_permission` origins, so the server gets a quick error instead of a
  hanging request.

## Security review checklist

- [ ] The content script responds only to `source: "seanime-page"` from `window` and the same
  origin. Iframes are ignored.
- [ ] The background accepts messages only from its own content scripts: `sender.id === runtime.id`,
  and `sender.origin` is in the connected origins.
- [ ] Host permissions are the only authorization for targets. Nothing is fetched for an origin the
  user didn't grant.
- [ ] No `eval`. No remote code. CSP is the MV3 default.
- [ ] No request or response bodies are persisted. `storage.local` holds only connected origins and
  preferences.
- [ ] A compromised Seanime server can make the browser fetch **granted** domains with the user's
  cookies. Document that grants should be limited to scraping targets, and never to a bank or
  email provider. The grant UI shows exactly what each provider requested.

## Tests

- **Vitest:** the guard (private IPs, permission check), challenge detection from fixture HTML,
  header rule building, the protocol drift check, and the body encoding choice.
- **Manual end-to-end:** `npm run build`, then load `dist/chrome` unpacked and `dist/firefox`
  through `about:debugging`.

## Distribution

- **CI:** `.github/workflows/browser-relay.yml` builds, tests, and zips `seanime-browser-relay-chrome.zip` and
  `-firefox.zip`, and attaches them to releases.
- **Chrome Web Store:** an unlisted listing. Store the link in `browser-relay/AGENTS.md` and in the
  web settings card constants.
- **AMO:** unlisted, signed with `web-ext sign --channel=unlisted`, which yields an installable
  `.xpi` link.
- **Versioning:** the extension version is independent. `hello` reports it, and the server rejects
  `v` ≠ 1 with a clear status message on the card.

## Commits

1. `feat(browser-relay): companion extension scaffold, build and connect flow`
2. `feat(browser-relay): relayed fetch with permission guard and header rules`
3. `feat(browser-relay): cloudflare/ddos-guard challenge solving in a background tab`
4. `ci(browser-relay): build, test and package the extension`
5. (later) `feat(browser-relay): render mode`
