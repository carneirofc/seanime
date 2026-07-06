# Seanime-Qt Settings — Design

**Date:** 2026-07-06
**Status:** Approved for implementation
**Scope:** A Settings screen for the seanime-qt PoC frontend covering both **client**
(app-local) preferences and the **server** settings surface exposed by the Go backend.

## Goal

Give seanime-qt a Settings page that mirrors the web app: edit the server's
`models.Settings` (Library, Media Player, Torrent, AniList, Discord, Manga,
Notifications, Nakama) and persist app-local connection/credentials so they survive
restarts. Stay Qt-only so the C++ port remains mechanical.

## Key backend facts

- `GET /api/v1/status` (already fetched on connect) returns the full `settings`
  object. **Reading server settings is free** — capture `data["settings"]` in
  `AppController._on_status`.
- `PATCH /api/v1/settings` accepts a body of 8 groups
  (`library, mediaPlayer, torrent, anilist, discord, manga, notifications, nakama`)
  and returns a fresh `Status`.
- **Wholesale bind:** the handler binds each group into a fresh struct, so any field
  omitted from the JSON is persisted as its Go zero value. Therefore every save MUST
  overlay edited fields onto the *current* group object (pass-through the rest); never
  send a partial group.

## Approach (chosen: A — QVariant dict + QML-assembled patch)

`AppController` exposes the settings dict as a single `QVariant` property. QML forms
read initial values via `app.settings.<group>.<field>` and, on Save, assemble the
8-group object (overlaying edits on `app.settings[group]`) and call
`app.saveServerSettings(obj)`. Minimal Python; all field knowledge lives in QML, which
ports to C++ verbatim; matches the app's existing "pass QVariant payloads" style.

## Components

### Python
- **`settings_store.py`** — `SettingsStore`, `QSettings`-backed (same pattern as
  `token_cache.py`). Persists client prefs: `serverHost`, `serverPort`, `serverToken`,
  `anilistClientId`, `anilistClientSecret`. Load at startup, save on change.
- **`api_client.py`** — add `settingsSaved = Signal("QVariant")`, `save_settings(body)`
  issuing `PATCH /api/v1/settings` via `QNetworkAccessManager.sendCustomRequest`, and a
  `_send_json(verb, path, body, on_success)` helper.
- **`app_controller.py`**
  - `_on_status`: capture `self._settings = data["settings"]`, emit `settingsChanged`.
  - `settings = Property("QVariant", …, notify=settingsChanged)`.
  - Client-pref properties `serverHost/serverPort/serverToken` (backed by the store,
    notify `clientPrefsChanged`); `anilistClientId/anilistClientSecret` backed by store.
  - `saveClientPrefs(host, port, token, clientId, clientSecret)` — persist all five,
    apply, reconnect.
  - `connectToServer` also persists host/port/token so top-bar connect is remembered.
  - `saveServerSettings(QVariant obj)` → `ApiClient.save_settings`.
  - `settingsSaved = Signal()` for a transient confirmation; `_on_settings_saved`
    refreshes `self._settings` from the returned Status.

### QML
- Reusable, themed field components: `SettingRow.qml` (label + description + control
  slot), `SettingSwitch.qml`, `SettingTextField.qml`, `SettingSpinBox.qml`,
  `SettingComboBox.qml`.
- **`SettingsView.qml`** — left sub-nav (Client, Library, Media Player, Torrent,
  AniList, Discord, Manga, Notifications, Nakama) + scrollable form per section + sticky
  Save bar. Server sections show a "connect to edit" hint when not connected. Repetitive
  boolean groups (AniList, Discord, Notifications) render via a `Repeater` over a local
  field list; Library/Media Player/Torrent/Nakama get bespoke layouts.
- **`Sidebar.qml`** — add `{ pageId: "settings", label: "Settings", glyph: "⚙️" }`.
- **`Main.qml`** — add `settingsComponent` + `showPage` wiring; initialize top-bar
  host/port/token from `app.serverHost/serverPort/serverToken`.

## Data flow

- **Read:** connect → `fetch_status` → `_on_status` captures `settings` →
  `settingsChanged` → form fields bind to `app.settings.<group>.<field>` (with `??`
  fallbacks).
- **Save server:** QML overlays edits on `app.settings[group]` → `saveServerSettings`
  → PATCH → returns Status → refresh `settings` + `settingsSaved` toast.
- **Save client:** Client section → `saveClientPrefs(...)` → QSettings write → reconnect.

## Error handling

Reuses the existing `errorOccurred` → banner path. Save failures surface there; forms
keep edited values (no reset on failure). Success emits `settingsSaved` for a transient
"Settings saved" confirmation.

## Verification

The PoC has no Python test suite; it verifies by running. Use the `seanime-qt` MCP
harness (`app_start`, `dump_tree`, `set_property`, `click`, `screenshot`): launch, open
Settings, toggle a field, save, confirm via a status refetch. Plus a manual pass against
a live local server.

## Out of scope

Theme switcher, title-language, and non-`/settings` config (mediastream, torrentstream,
debrid, auto-downloader rules) — separate endpoints/screens.
