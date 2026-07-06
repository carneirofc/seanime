# Torrent Download for Library Items — Design

**Date:** 2026-07-06
**Project:** seanime-qt (PySide6/QML desktop client for the Seanime server)
**Status:** Approved (design), pending implementation plan

## Goal

Let a user browse torrents for a library anime and send a chosen torrent to their
configured torrent client (qBittorrent/Transmission), downloading into the library.
Entry points from both the anime detail page and individual episode rows.

## Non-goals (out of scope for this POC)

- Multi-torrent selection.
- Per-file selection within a torrent ("choose files to download").
- Debrid service downloads.
- Downloading `.torrent` files to the client machine.

These are noted as possible future work but are explicitly excluded here.

## Context

seanime-qt is a thin client. The Seanime **server** already implements all torrent
logic and exposes it over HTTP:

- `POST /api/v1/torrent/search` — body `{type, provider, query, episodeNumber, batch,
  media, resolution}`; returns `SearchData` with `torrents: []AnimeTorrent`. When
  `provider` is empty the server falls back to the user's **default** torrent provider
  (`internal/torrents/torrent/search.go:432`). So the client needs no provider picker.
- `POST /api/v1/torrent-client/download` — body `{torrents, destination, smartSelect,
  media}`; adds the torrent(s) to the configured client. `destination` must be an
  **absolute** path and should sit within a library path (server enforces).

`AnimeTorrent` fields used by the UI: `name`, `formattedSize`/`size`, `seeders`,
`leechers`, `resolution`, `releaseGroup`, `isBatch`, `isBestRelease`, `confirmed`,
`date`, `link`, `magnetLink`, `infoHash`, `downloadUrl`
(`internal/extension/hibike/torrent/types.go:122`).

The web client's destination logic (ported here) lives in
`seanime-web/.../torrent-search/torrent-download-file-selection.tsx`:

```
getDefaultDestination(entry, libraryPath):
  if entry.localFiles has any -> dirname(last local file path)
  else if libraryPath        -> join(libraryPath, sanitize(media.title.romaji))
  else                       -> ""
```

`libraryPath` is read from server status `settings.library.libraryPath`, already
available to the client via `/api/v1/status` (`AppController._settings`).

## Architecture

UI + API plumbing only; no torrent logic in the client.

```
[DetailHeader "Download" button]  [EpisodeDelegate download icon]
                 \                    /
              AppController.openTorrentSearch(episodeNumber=-1, batch=False)
                            |
                emit torrentSearchOpened  -> Main.qml stack.push(TorrentSearchView)
                            |
   TorrentSearchView: query / resolution / batch / episode controls
                            |
     AppController.runTorrentSearch(...) -> ApiClient.search_torrent(payload)
                            |
        POST /api/v1/torrent/search -> torrentSearchReceived -> TorrentModel.load()
                            |
             TorrentDelegate row click -> AppController.selectTorrent(index)
                            |
   compute default destination + smart-select eligibility; emit torrentDownloadReady
                            |
        DownloadConfirmDialog: editable destination, Download [/ Download missing]
                            |
   AppController.startTorrentDownload(destination, smartSelect)
                            |
   ApiClient.torrent_client_download(payload) -> POST /api/v1/torrent-client/download
                            |
          torrentDownloadSucceeded -> close dialog + return to detail
```

## Components

### ApiClient (`src/seanime_qt/api_client.py`)

Two methods using the existing `_post_json(path, body, on_success)` helper:

- `search_torrent(payload: dict)` -> emits `torrentSearchReceived("QVariant")`
- `torrent_client_download(payload: dict)` -> emits `torrentDownloadSucceeded("QVariant")`

Failures route through the existing `errorOccurred` signal (default error path of
`_post_json`).

### TorrentModel (`src/seanime_qt/torrent_model.py`, new)

`QAbstractListModel` following the `CharacterModel` pattern (role enum -> byte-name
map -> `load()` -> `data`/`rowCount`/`roleNames`).

Roles (QML-visible): `name`, `formattedSize`, `seeders`, `leechers`, `resolution`,
`releaseGroup`, `isBatch`, `isBestRelease`, `confirmed`, `date`, `link`.

Internally keeps the raw torrent dicts. Exposes `@Slot(int, result="QVariant")
torrentAt(index)` so the full torrent object can be passed back to the download call.
`load(search_data)` reads `search_data["torrents"]` and formats size when
`formattedSize` is empty.

### destination.py (`src/seanime_qt/destination.py`, new — pure functions)

Ported from the web app; no Qt imports, so unit-testable in isolation.

- `sanitize_directory_name(name: str) -> str` — replace `<>:"/\|?*` and control chars
  with spaces, collapse whitespace, strip leading/trailing dots and spaces, fall back
  to `"Untitled"`.
- `default_destination(local_files: list, library_path: str, romaji_title: str) -> str`
  — dirname of the last local file if any, else `join(library_path, sanitize(romaji))`,
  else `""`. Uses `os.path`/`posixpath` normalization consistent with server paths.

### AppController (`src/seanime_qt/app_controller.py`)

Data capture:
- In `_on_anime_entry` (currently discards most of the payload at line ~1184), stash
  raw `self._entry_media` (the `media` object / BaseAnime), `self._entry_local_files`
  (`data["localFiles"]`), and `self._entry_download_info` (`data["downloadInfo"]`).
- `library.libraryPath` is already reachable via `self._settings`.

State + `Property` exposure (QML):
- `torrentModel` (the `TorrentModel` instance)
- `torrentSearchLoading: bool`, `torrentSearchError: str`
- `torrentSelectedName: str`, `torrentSelectedIsBatch: bool`
- `torrentCanSmartSelect: bool`
- `torrentDefaultDestination: str`
- current search context: `torrentSearchEpisode: int`, `torrentSearchBatch: bool`

Methods (`@Slot`):
- `openTorrentSearch(episodeNumber: int = -1, batch: bool = False)` — sets context,
  clears the model, emits `torrentSearchOpened`, and kicks off an initial
  `runTorrentSearch`.
- `runTorrentSearch(query: str, episodeNumber: int, batch: bool, resolution: str)` —
  builds the search payload (`type="smart"`, `provider=""`, `media=self._entry_media`)
  and calls `ApiClient.search_torrent`. Sets `torrentSearchLoading`. Search type is
  always `"smart"` (the anime-aware episode/batch/resolution filtering is the point of
  this feature). If the user's default provider does not support smart search, the
  server returns an error that surfaces via the error banner; a `"simple"`/raw-query
  fallback is deferred to future work.
- `selectTorrent(index: int)` — stash `torrentAt(index)`; compute
  `torrentDefaultDestination` via `default_destination(...)`; compute
  `torrentCanSmartSelect`; emit `torrentDownloadReady`.
- `startTorrentDownload(destination: str, smartSelect: bool)` — validate destination
  (non-empty, absolute); build payload `{torrents: [selected], destination, media,
  smartSelect: {enabled, missingEpisodeNumbers}}`; call
  `ApiClient.torrent_client_download`.

Signals: `torrentSearchOpened`, `torrentDownloadReady`, `torrentDownloadStarted`,
`torrentSearchModelChanged` (or reuse a notify property).

Smart-select eligibility (mirrors web `canSmartSelect`):
`selected.isBatch and media.format != "MOVIE" and media.status == "FINISHED"
and media.episodes > 1 and downloadInfo.episodesToDownload is non-empty
and len(episodesToDownload) != total episodes`.

### QML

- `qml/TorrentSearchView.qml` (new) — mirrors `SearchView.qml`. Top: `AppTextField`
  query, `AppComboBox` resolution (Any/1080/720/540/480), `AppSwitch` batch toggle,
  optional episode field (shown/pre-filled when launched per-episode), a "Search"
  action. Body: `ListView` of `TorrentDelegate` bound to `app.torrentModel`, with
  loading and empty states. Back button pops the stack.
- `qml/TorrentDelegate.qml` (new) — one result row: name (elided), a row of chips for
  resolution / size / seeders (green) / leechers / batch / best-release / confirmed,
  and date. Click -> `app.selectTorrent(index)`.
- `qml/DownloadConfirmDialog.qml` (new) — a `Dialog`/`Popup` with an editable
  destination `AppTextField` pre-filled from `app.torrentDefaultDestination`, the
  selected torrent name, a primary "Download" `AppButton`, and — when
  `app.torrentCanSmartSelect` — a secondary "Download missing episodes" button. Buttons
  disable while a request is in flight.
- `qml/DetailHeader.qml` — add a "Download" `AppButton` near the list-edit button,
  calling `app.openTorrentSearch()`.
- `qml/EpisodeDelegate.qml` — add a small download icon-button calling
  `app.openTorrentSearch(episodeNumber, false)`.
- `qml/Main.qml` — handle `onTorrentSearchOpened` -> `stack.push(torrentSearchComponent)`;
  host the `DownloadConfirmDialog` and open it on `onTorrentDownloadReady`.

## Error handling

- No torrent client configured / provider missing / download rejected: server returns
  an error -> existing `errorOccurred` -> error banner.
- Download button disabled while a request is in flight (`torrentSearchLoading` / a
  download-in-flight flag).
- Empty or relative destination blocked client-side before sending; server also
  enforces absolute + library-path.

## Testing

The project has no existing test suite. Add a minimal `pytest` module,
`seanime-qt/tests/test_destination.py`, covering the pure functions in
`destination.py`:

- `sanitize_directory_name`: illegal chars replaced, whitespace collapsed, leading/
  trailing dots/spaces stripped, empty -> `"Untitled"`.
- `default_destination`: local-files present -> dirname of last file; absent + library
  path -> joined sanitized title; absent + no library path -> `""`.

The end-to-end UI flow (button -> search results -> confirm dialog pre-fill -> download
call fires) is verified manually against the running app via the MCP agent tools.

## Files touched

New:
- `src/seanime_qt/torrent_model.py`
- `src/seanime_qt/destination.py`
- `qml/TorrentSearchView.qml`
- `qml/TorrentDelegate.qml`
- `qml/DownloadConfirmDialog.qml`
- `tests/test_destination.py`

Modified:
- `src/seanime_qt/api_client.py` (two methods + two signals)
- `src/seanime_qt/app_controller.py` (state, props, slots, signals, entry-payload capture)
- `qml/DetailHeader.qml` (Download button)
- `qml/EpisodeDelegate.qml` (download icon-button)
- `qml/Main.qml` (push view, host confirm dialog)
- `CHANGELOG.md` (Added entry, Keep a Changelog format)
